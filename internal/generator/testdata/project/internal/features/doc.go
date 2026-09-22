// Package features contains one vertical slice per feature of testapp.
//
// Each subpackage owns everything its feature needs — HTTP handler, business
// logic, storage and tests — so a change to one feature never forces a change
// in another. Run `feather new feature <name>` to add one to this package.
package features
