package auth

import (
	"log"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestMakeJWT(t *testing.T) {

	log.Print("TEstMakeJWT...")

	newId := uuid.New()
	secret := "secret"
	duration, err := time.ParseDuration("1000")
	if err != nil {

	}
	token, err := MakeJWT(newId, secret, duration)
	if err != nil {
		t.Errorf("FAIL")
	}

	log.Printf("ok token: %s", token)

}

func TestValidateJWT(t *testing.T) {
	log.Print("TestValidateJWT...")
	newId := uuid.New()
	secret := "secret"
	duration, err := time.ParseDuration("1000")
	if err != nil {

	}
	token, err := MakeJWT(newId, secret, duration)
	if err != nil {
		t.Errorf("FAIL")
	}

	uid, err := ValidateJWT(token, secret)
	if err != nil {
		t.Errorf("FAIL")
	}

	log.Printf("ok uid: %s", uid)
}

func TestInValidJWT_ShouldFail(t *testing.T) {
	log.Print("TestInvalidJWT...")
	newId := uuid.New()
	secret := "secret"
	duration, err := time.ParseDuration("1000")
	if err != nil {

	}
	token, err := MakeJWT(newId, secret, duration)
	if err != nil {
		t.Errorf("FAIL")
	}

	invalid_secret := "whatever"
	uid, err := ValidateJWT(token, invalid_secret)
	if err == nil {
		log.Printf("uid = %s", uid)
		t.Fatal("FAIL expected an error, but got nil")
	}

}
