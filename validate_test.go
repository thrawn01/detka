package detka_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/thrawn01/detka"
)

var _ = Describe("Validate", func() {
	Describe("ValidEmail", func() {
		Context("When address is missing @", func() {
			It("should return an error", func() {
				err := detka.ValidEmail("derrick at google.com")
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(Equal("'derrick at google.com': mail: missing phrase"))
			})
		})
		Context("When address is empty string", func() {
			It("should return an error", func() {
				err := detka.ValidEmail("")
				Expect(err).To(Not(BeNil()))
				Expect(err.Error()).To(Equal("'': mail: no address"))
			})
		})
		Context("When address is supplied", func() {
			It("should return nil", func() {
				err := detka.ValidEmail("Derrick <thrawn01@gmail.com>")
				Expect(err).To(BeNil())

				err = detka.ValidEmail("thrawn01@gmail.com")
				Expect(err).To(BeNil())
			})
		})
		Context("When comma separated list of addresses is supplied", func() {
			It("should return nil", func() {
				err := detka.ValidEmail("Derrick <thrawn01@gmail.com>, John <john@gmail.com>")
				Expect(err).To(BeNil())

				err = detka.ValidEmail("thrawn01@gmail.com, john@gmail.com")
				Expect(err).To(BeNil())
			})
		})
	})
})
