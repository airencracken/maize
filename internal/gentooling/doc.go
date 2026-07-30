// Package gentooling translates in-process github.com/airencracken/gentooling
// library results into Maize domain types.
//
// General package-manager behavior belongs in Gentooling. This package must not
// invoke Arise, Portage, or package-query commands as subprocesses. When an
// operation is unavailable, the correct change is to expose it from Gentooling
// and then consume that API here.
package gentooling
