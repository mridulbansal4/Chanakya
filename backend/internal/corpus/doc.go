// Package corpus holds the CI assertions over the checked-in testing corpus at
// testdata/.
//
// SAFETY ROLE. These are a regression suite on the DEMO NARRATIVE, not only on
// the code. If a fixture refactor quietly turns the 118-client MITC gap into
// 117, everything still compiles and every other test still passes while the
// product's central claim has silently changed. That is the failure this package
// exists to catch. It also carries the adversarial prompt-injection test, which
// proves the schema and citation gates hold at the ingestion boundary.
//
// The package has no non-test code: it is assertions about data that lives
// elsewhere.
package corpus
