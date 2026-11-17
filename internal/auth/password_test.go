package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("Sup3rSecreta")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("formato de hash inesperado: %s", hash)
	}
	if hash == "Sup3rSecreta" || strings.Contains(hash, "Sup3rSecreta") {
		t.Fatal("la contraseña no debe aparecer en el hash")
	}
	if !VerifyPassword(hash, "Sup3rSecreta") {
		t.Fatal("la contraseña correcta no valida")
	}
	if VerifyPassword(hash, "Sup3rSecret4") {
		t.Fatal("una contraseña incorrecta no debe validar")
	}
	if VerifyPassword("basura", "Sup3rSecreta") {
		t.Fatal("un hash inválido no debe validar")
	}
}

func TestHashIsSalted(t *testing.T) {
	a, err := HashPassword("MismaClave123")
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword("MismaClave123")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("dos hashes de la misma contraseña deben diferir por la sal")
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	if err := ValidatePassword("corta1"); err == nil {
		t.Fatal("se esperaba error por longitud")
	}
	if err := ValidatePassword("solamenteletras"); err == nil {
		t.Fatal("se esperaba error por falta de dígitos")
	}
	if err := ValidatePassword("1234567890"); err == nil {
		t.Fatal("se esperaba error por falta de letras")
	}
	if err := ValidatePassword("ClaveSegura1"); err != nil {
		t.Fatalf("una contraseña válida fue rechazada: %v", err)
	}
}

func TestTokens(t *testing.T) {
	a, err := NewToken(32)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || len(a) < 20 {
		t.Fatalf("tokens no aleatorios: %q %q", a, b)
	}
	if HashToken(a) == a || HashToken(a) != HashToken(a) {
		t.Fatal("HashToken debe ser determinista y no reversible")
	}
}
