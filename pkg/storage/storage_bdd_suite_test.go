package storage_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStorageBDD(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage BDD Suite")
}
