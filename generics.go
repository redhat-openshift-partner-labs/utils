package utils

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator"
	"github.com/google/uuid"
	"google.golang.org/api/docs/v1"
	"log"
	"os"
)

var (
	// BitcoinAlphabet is the bitcoin alphabet.
	BitcoinAlphabet, _ = NewAlphabet("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")

	// FlickrAlphabet is the flickr alphabet.
	FlickrAlphabet, _ = NewAlphabet("123456789abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ")
)

func ErrorCheck(message string, err error) (ok bool) {
	if err != nil {
		log.Printf("%s: %v", message, err)
	}
	return true
}

func Contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func RemoveArtifacts(artifacts []string) {
	for _, artifact := range artifacts {
		err := os.Remove(artifact)
		ErrorCheck("Unable to remove file: %v", err)
	}
}

func getDocStatus(service *docs.Service, id string) int {
	docStatus, err := service.Documents.Get(id).Do()
	ErrorCheck("Unable to make Get call for Google Doc: ", err)
	return docStatus.HTTPStatusCode
}

func NewAlphabet(src string) (*Alphabet, error) {
	if len(src) != 58 {
		return nil, fmt.Errorf("invalid alphabet: base58 alphabets must be 58 bytes long")
	}

	var alphabet Alphabet
	copy(alphabet.Encode[:], src)

	for i := range alphabet.Decode {
		alphabet.Decode[i] = -1
	}

	for i, b := range alphabet.Encode {
		alphabet.Decode[b] = int8(i)
	}

	return &alphabet, nil
}

func EncodeAlphabet(src []byte, alphabet *Alphabet) string {
	zero := alphabet.Encode[0]
	srcSize := len(src)
	var i, j, zcount, high, carry int

	for zcount < srcSize && src[zcount] == 0 {
		zcount++
	}

	size := (srcSize-zcount)*138/100 + 1
	buf := make([]byte, size*2+zcount)

	tmp := buf[size+zcount:]

	high = size - 1
	for i = zcount; i < srcSize; i++ {
		j = size - 1
		for carry = int(src[i]); j > high || carry != 0; j-- {
			carry = carry + 256*int(tmp[j])
			tmp[j] = byte(carry % 58)
			carry /= 58
		}
		high = j
	}

	for j = 0; j < size && tmp[j] == 0; j++ {
	}

	b58 := buf[:size-j+zcount]

	if zcount != 0 {
		for i = 0; i < zcount; i++ {
			b58[i] = zero
		}
	}

	for i = zcount; j < size; i++ {
		b58[i] = alphabet.Encode[tmp[j]]
		j++
	}

	return string(b58)
}

func Encode(src []byte) string {
	return EncodeAlphabet(src, BitcoinAlphabet)
}

func DecodeAlphabet(src string, alphabet *Alphabet) ([]byte, error) {
	if len(src) == 0 {
		return nil, fmt.Errorf("invalid encoded srcing: length must be greater than 0")
	}

	var (
		t, c      uint64
		zmask     uint32
		zcount    int
		b58u      = []rune(src)
		b58sz     = len(b58u)
		outisz    = (b58sz + 3) >> 2
		binu      = make([]byte, (b58sz+3)*3)
		bytesleft = b58sz & 3
		zero      = rune(alphabet.Encode[0])
	)

	if bytesleft > 0 {
		zmask = 0xffffffff << uint32(bytesleft*8)
	} else {
		bytesleft = 4
	}

	var outi = make([]uint32, outisz)

	for i := 0; i < b58sz && b58u[i] == zero; i++ {
		zcount++
	}

	for _, r := range b58u {
		if r > 127 {
			return nil, fmt.Errorf("high-bit set on invalid digit")
		}

		if alphabet.Decode[r] == -1 {
			return nil, fmt.Errorf("invalid base58 digit (%q)", r)
		}

		c = uint64(alphabet.Decode[r])

		for j := outisz - 1; j >= 0; j-- {
			t = uint64(outi[j])*58 + c
			c = (t >> 32) & 0x3f
			outi[j] = uint32(t & 0xffffffff)
		}

		if c > 0 {
			return nil, fmt.Errorf("output number too big (carry to the next int32)")
		}

		if outi[0]&zmask != 0 {
			return nil, fmt.Errorf("output number too big (last int32 filled too far)")
		}
	}

	var j, cnt int

	for j, cnt = 0, 0; j < outisz; j++ {
		for mask := byte(bytesleft-1) * 8; mask <= 0x18; mask, cnt = mask-8, cnt+1 {
			binu[cnt] = byte(outi[j] >> mask)
		}
		if j == 0 {
			bytesleft = 4
		}
	}

	for n, v := range binu {
		if v > 0 {
			start := n - zcount
			if start < 0 {
				start = 0
			}
			return binu[start:cnt], nil
		}
	}

	return binu[:cnt], nil
}

func Decode(src string) ([]byte, error) {
	return DecodeAlphabet(src, BitcoinAlphabet)
}

func Validate(CurrentLabRequest string) LabRequest {
	// create labRequest struct for this validation request
	var labRequest LabRequest

	// unmarshal the incoming lab request and put into the struct
	err := json.Unmarshal([]byte(CurrentLabRequest), &labRequest)
	if err != nil {
		log.Fatal(err)
	}

	// generate a lab ID and set the labRequest.ID field to generated UUID
	labRequest.ID = uuid.New()

	// validate the request
	validate := validator.New()
	err = validate.Struct(labRequest)
	if err != nil {
		fmt.Printf("Unable to validate the request: %v", err)
		log.Fatal(err)
	}

	return labRequest
}
