// Package arise translates in-process Arise library results into Maize domain
// types.
//
// General package-manager behavior belongs in Arise. This package must not
// invoke Arise, Portage, or package-query commands as subprocesses. When an
// operation is unavailable, the correct change is to expose it from an Arise
// Go library and then consume that API here.
package arise
