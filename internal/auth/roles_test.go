package auth

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

func newAuthService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	box, err := cryptoutil.LoadOrCreate(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "test.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(st, config.Security{Auth: true, SessionHours: 1}, nil), st
}

func createUser(t *testing.T, svc *Service, name, role string) *model.User {
	t.Helper()
	u, err := svc.CreateUser(name, "CorrectHorse9", role)
	if err != nil {
		t.Fatalf("crear %s: %v", name, err)
	}
	return u
}

func TestValidRole(t *testing.T) {
	for _, role := range []string{"admin", "owner", "operator", "viewer"} {
		if !ValidRole(role) {
			t.Errorf("rol %q debería ser válido", role)
		}
	}
	for _, role := range []string{"", "root", "Admin", "readonly"} {
		if ValidRole(role) {
			t.Errorf("rol %q no debería ser válido", role)
		}
	}
}

func TestCreateUserRoleValidation(t *testing.T) {
	svc, _ := newAuthService(t)
	if _, err := svc.CreateUser("bad", "CorrectHorse9", "root"); err == nil {
		t.Fatal("se aceptó un rol inválido")
	}
	u := createUser(t, svc, "viewer1", "viewer")
	if u.Role != "viewer" {
		t.Fatalf("rol inesperado: %q", u.Role)
	}
	u2 := createUser(t, svc, "ops", "operator")
	if u2.Role != "operator" {
		t.Fatalf("rol inesperado: %q", u2.Role)
	}
}

func TestUpdateUserRoleRefusesLastAdmin(t *testing.T) {
	svc, st := newAuthService(t)
	admin := createUser(t, svc, "admin", "admin")

	if _, err := svc.UpdateUserRole(admin.ID, "viewer"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("se esperaba ErrLastAdmin, se obtuvo %v", err)
	}
	u, err := st.GetUser(admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if u.Role != "admin" {
		t.Fatalf("el último admin cambió de rol a %q", u.Role)
	}
}

func TestUpdateUserRoleAndSessionInvalidation(t *testing.T) {
	svc, st := newAuthService(t)
	createUser(t, svc, "admin", "admin")
	second := createUser(t, svc, "admin2", "admin")
	viewer := createUser(t, svc, "viewer", "viewer")

	// Sessions must be dropped when the role changes.
	sess, _, err := svc.Login("viewer", "CorrectHorse9", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Validate(sess.ID); err != nil {
		t.Fatalf("la sesión recién creada no valida: %v", err)
	}
	updated, err := svc.UpdateUserRole(viewer.ID, "operator")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Role != "operator" {
		t.Fatalf("rol inesperado: %q", updated.Role)
	}
	if _, _, err := svc.Validate(sess.ID); err == nil {
		t.Fatal("la sesión antigua sigue siendo válida tras cambiar el rol")
	}

	// Now two admins exist, so one can be demoted.
	if _, err := svc.UpdateUserRole(second.ID, "viewer"); err != nil {
		t.Fatalf("degradar con dos admins: %v", err)
	}
	if n, _ := st.CountAdmins(); n != 1 {
		t.Fatalf("administradores restantes: %d", n)
	}
}

func TestDeleteUserGuardsLastAdmin(t *testing.T) {
	svc, st := newAuthService(t)
	admin := createUser(t, svc, "admin", "admin")
	viewer := createUser(t, svc, "viewer", "viewer")

	if err := svc.DeleteUser(admin.ID); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("se esperaba ErrLastAdmin, se obtuvo %v", err)
	}
	if _, err := st.GetUser(admin.ID); err != nil {
		t.Fatalf("el último admin desapareció: %v", err)
	}
	if err := svc.DeleteUser(viewer.ID); err != nil {
		t.Fatalf("eliminar viewer: %v", err)
	}
	if _, err := st.GetUser(viewer.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("el viewer sigue existiendo: %v", err)
	}
}

func TestResetPasswordLogsOutSessions(t *testing.T) {
	svc, st := newAuthService(t)
	createUser(t, svc, "admin", "admin")
	u := createUser(t, svc, "user1", "operator")

	sess, _, err := svc.Login("user1", "CorrectHorse9", "127.0.0.1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(u.ID, "AnotherPass123"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Validate(sess.ID); err == nil {
		t.Fatal("la sesión sobrevivió al restablecimiento de contraseña")
	}
	_, hash, err := st.GetUserWithHash("user1")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "AnotherPass123") {
		t.Fatal("la contraseña nueva no valida")
	}
	if err := svc.ResetPassword(u.ID, "short"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("se esperaba ErrWeakPassword, se obtuvo %v", err)
	}
}
