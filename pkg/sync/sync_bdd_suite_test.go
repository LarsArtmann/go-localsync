package sync_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSyncBDD(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sync BDD Suite")
}
