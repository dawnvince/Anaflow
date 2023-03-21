package util

import (
	"fmt"
)

func CheckError(err error) bool{
	if err != nil {
		fmt.Printf("Error %s\n", err.Error())
		return true
	}
	return false
}

func IPbyte2int(ip []byte) (ipint uint32) {
	var t uint32
	for _, c := range ip {
		if c == '.' {
			ipint += t
			t = 0
			ipint <<= 8
		} else if c >= '0' && c <= '9' {
			t *= 10
			t += uint32(c - '0')
		} else {
			fmt.Println("Wrong IP format!!")
		}
	}
	return ipint + t
}