package main

import (
	"errors"
	"fmt"
	"reflect"
)

type csIntf interface {
	F()
}

type bcIntf interface {
	G()
	F()
	ReturnIntf2(t reflect.Type) (interface{}, error)
}

type bcStruct struct {
	A int
	B string
}

func (s *bcStruct) G() {
	fmt.Println("G")
}

func (s *bcStruct) F() {
	fmt.Println("F")
}

func (s *bcStruct) ReturnIntf2(t reflect.Type) (interface{}, error) {
	if !reflect.TypeOf(s).AssignableTo(t) {
		return nil, errors.New("damm, not compatible")
	}
	return s, nil
}

func main() {
	a := bcStruct{A: 1, B: "abc"}
	var b bcIntf

	b = &a

	fmt.Println(a, b)

	csIntfType := reflect.TypeOf((*csIntf)(nil)).Elem()

	b1, err := b.ReturnIntf2(csIntfType)
	if err != nil {
		fmt.Println(err)
		return
	}

	b1.(csIntf).F()
	fmt.Println("end")
}
