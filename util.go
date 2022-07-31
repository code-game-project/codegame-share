package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var (
	ErrDecodeRequestBody = errors.New("decode-request-body")
	ErrInvalidFields     = errors.New("invalid-fields")
)

func decodeRequestBody(r *http.Request, target any) error {
	err := json.NewDecoder(r.Body).Decode(&target)
	r.Body.Close()
	if err != nil {
		fmt.Println(err)
		return ErrDecodeRequestBody
	}

	v := validator.New()
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	err = v.Struct(target)

	vErrs, ok := err.(validator.ValidationErrors)
	if ok && len(vErrs) > 0 {
		errs := vErrs[0].Field()
		for i := 1; i < len(vErrs); i++ {
			errs = fmt.Sprintf("%s,%s", errs, vErrs[i].Field())
		}
		return fmt.Errorf("%w:%s", ErrInvalidFields, errs)
	}

	return nil
}

func respond(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)

	err := json.NewEncoder(w).Encode(payload)
	if err != nil {
		log.Print("Failed to encode response: ", err)
	}
}

func respondError(w http.ResponseWriter, code int, error string) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(code)

	type response struct {
		Error string `json:"error"`
	}
	resp := response{
		Error: error,
	}

	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Print("Failed to encode response: ", err)
	}
}

func respondDecodeError(w http.ResponseWriter, decodeError error) {
	if !errors.Is(decodeError, ErrInvalidFields) {
		respondError(w, http.StatusBadRequest, decodeError.Error())
		return
	}

	parts := strings.Split(decodeError.Error(), ":")
	if len(parts) != 2 {
		log.Print("Invalid error passed into 'respondDecodeError'. Using 'respondError' instead...")
		respondError(w, http.StatusInternalServerError, decodeError.Error())
		return
	}

	type response struct {
		Error  string   `json:"error"`
		Fields []string `json:"fields"`
	}
	resp := response{
		Error:  parts[0],
		Fields: strings.Split(parts[1], ","),
	}

	respond(w, http.StatusBadRequest, resp)
}
