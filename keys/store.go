package keys

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

type KeyRing struct {
	Keys   []APIKey
	Primed *APIKey
	Active *APIKey
}

func (kr *KeyRing) Generate() error {
	k := new(APIKey)
	k.ID = fmt.Sprintf("%x", time.Now().UnixNano())
	k.Status = "Primed"
	b := make([]byte, 48)
	if _, err := rand.Read(b); err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}
	k.Key = base64.RawURLEncoding.EncodeToString(b)
	k.GeneratedAt = time.Now().Format(time.RFC3339)
	kr.Primed = k
	return nil
}

func (kr *KeyRing) Status() {
	fmt.Printf("Primed: %v\n", kr.Primed)
	fmt.Printf("Active: %v\n", kr.Active)
}

func (kr *KeyRing) Use() error {
	if kr.Primed == nil {
		return fmt.Errorf("No key primed")
	}
	kr.Active = kr.Primed
	kr.Primed = nil
	return nil
}
