package validation

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsValidHost(t *testing.T) {
	for _, i := range []struct {
		s     string
		valid bool
	}{
		{"example.com", true},
		{"example.org", true},
		{"foobar.example.com", true},
		{"foobar.example.com_", false},
		{"foobar.example.com[", false},
		{"foobar.example.com]", false},
		{"foobar.example.com<", false},
		{"foobar.example.com/", false},
		{"foobar.example.com?", false},
		{"foobar.example.com@", false},
		{"foobar.example.com!", false},
		{"foobar.example..com", false},
		{"foobar.example-.com", false},
		{"foobar.-example.com", false},
		{"-foobar.example.com", false},
		{"foobar.example.com-", false},
		{"foobar..example..com", false},
		{".example.com", false},
		{"example.com.", false},
	} {
		assert.Equal(t, i.valid, IsValidHost(i.s), "Test failed \"%s\" - %v", i.s, i.valid)
	}
}

func TestIsValidSite(t *testing.T) {
	for _, i := range []struct {
		s     string
		valid bool
	}{
		{"", false},
		{"a", true},
		{"abc", true},
		{"0", true},
		{"0123456789", true},
		{"_", true},
		{"_ab", true},
		{"-", false},
		{"-ab", false},
		{".", true},
		{".ab", true},
		{"a-b", true},
		{"a_b", true},
		{"a/b", false},
		{"a.b", true},
		{"ab-", true},
		{"ab_", true},
		{"ab.", true},
		{"/ab", false},
		{"ab/", false},
	} {
		assert.Equal(t, i.valid, IsValidSite(i.s), "Test failed \"%s\" - %v", i.s, i.valid)
	}
}

func TestIsValidBranch(t *testing.T) {
	for _, i := range []struct {
		s     string
		valid bool
	}{
		{"", false},
		{"a", true},
		{"abc", true},
		{"0", true},
		{"0123456789", true},
		{"_", true},
		{"_ab", true},
		{"-", false},
		{"-ab", false},
		{".", true},
		{".ab", true},
		{"a-b", true},
		{"a_b", true},
		{"a/b", true},
		{"a.b", true},
		{"ab-", true},
		{"ab_", true},
		{"ab.", true},
		{"/ab", false},
		{"ab/", false},
	} {
		assert.Equal(t, i.valid, IsValidBranch(i.s), "Test failed \"%s\" - %v", i.s, i.valid)
	}
}
