package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHashPass(t *testing.T) {
	testPass := "PurpleMonkeyDishWasher"
	hashed, err := HashPassword(testPass)
	if err != nil {
		t.Errorf("HashPassword() returned error %v", err)
		return
	}
	if err = CheckPasswordHash(testPass, hashed); err != nil {
		t.Errorf("Error checking hashed password: %v", err)
	}
}

// ------------ 123456789012345678901234567890123456789012345678901234567890123456789012345678901234567
const secret = "hiT6/qkbcpGn8LokB7qLxgNDADBn1IvjBtB2W7iMd84vXebR8Vd2TGDs2NURfVebJISiBsE16txLRV8xt9GnSQ=="

func TestJWTLoop(t *testing.T) {
	testUuid := uuid.New()
	tokenstr, err := MakeJWT(testUuid, secret, time.Hour)
	if err != nil {
		t.Errorf("MakeJWT() returned error %v", err)
		return
	}
	backUuid, err := ValidateJWT(tokenstr, secret)
	if err != nil {
		t.Errorf("ValidateJWT() returned error %v", err)
		return
	}
	if testUuid != backUuid {
		t.Errorf("Oroginal UUID %v does not match retrieved UUID %v", testUuid, backUuid)
	}
}

func TestJWTTimeout(t *testing.T) {
	testUuid := uuid.New()
	tokenstr, err := MakeJWT(testUuid, secret, time.Second)
	if err != nil {
		t.Errorf("MakeJWT() returned error %v", err)
		return
	}
	time.Sleep(3 * time.Second)
	_, err = ValidateJWT(tokenstr, secret)
	if err == nil {
		t.Errorf("ValidateJWT() should have returned timeout error")
	} else {
		t.Logf("ValidateJWT() returned error that should indicate time-out: %v", err)
	}

}
