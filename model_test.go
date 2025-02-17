package main

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
)

func TestSomething(t *testing.T) {
	previewReq := PreviewRequest{
		Configuration: PreviewConfiguration{
			Template: "something",
		},
	}
	val := validator.New()
	err := val.Struct(previewReq)
	assert.Nil(t, err)
}
