# LRU cache code generator for Go

[![License: BSD 3 Clause](https://img.shields.io/badge/License-BSD_3--Clause-yellow.svg)](https://opensource.org/licenses/BSD-3-Clause)

_Note:_ A version of this cache written using Go generics is [here](https://github.com/maxim2266/cache).

### Invocation

Example:
```sh
gen-cache --name UserInfoCache --key int --value '*UserInfo' \
	--package main --output 'user_info_cache.go'
```
This command generates source code for a cache that stores values of type `*UserInfo`
with keys of type `int`. The cache object is named `UserInfoCache`, it resides in the package `main`,
and the generated code is stored in the file `user_info_cache.go`.

In general, the command has the following form:
```
gen-cache -k/--key TYPE -v/--value TYPE -n/--name NAME -p/--package NAME [-o/--output FILE]
```
where `TYPE` may be any valid Go type (although for the key the type must be acceptable as a key for
Go `map`), `NAME` should be a valid Go identifier, and `FILE` may be any file name. The last parameter
is optional, and if omitted the code will be written to `stdout`.

Typically, the code generator is invoked using `//go:generate` command from a Go source file.

### API

For any given types `K` (for the key) and `V` (for the value), and any given name `${name}`,
the generated code will define:

* An opaque type `${name}` to represent the cache. There may be multiple caches in one Go
package, as long as their names are different.

* A cache constructor in the form
	```Go
	func [Nn]ew${name}(size int, ttl time.Duration,
	                   backend func(K) (V, error)) *${name}
	```
	where the first letter of the function name is capital if the first letter of the given name
	is also capital, to follow Go visibility rules. For example, if `K` is `int`, `V` is `*UserInfo`,
	and the name is `UserInfoCache`, then the constructor function will be generated as
	```Go
	func NewUserInfoCache(size int, ttl time.Duration,
	                      backend func(int) (*UserInfo, error)) *UserInfoCache
	```
	Constructor parameters:
	* Maximum size of the cache (a positive integer);
	* Time-to-live for cache elements (can be set to something like one year if not needed);
	* Back-end function to call when a cache miss occurs. The function is expected to return a value
		for the given key, or an error. Both the value _and_ the error are stored in the cache.
		A slow back-end function is not going to block access to the entire cache, only to the
		corresponding value.

	The constructor returns a pointer to a newly created cache object.

A cache object has two (public) methods:
* `Get(K) (V, error)`: given a key, it returns the corresponding value, or an error. On cache miss
the result is transparently retrieved from the back-end. The cache itself does not produce any error,
so all the errors are from the back-end. Notably, this method has the same signature as the
back-end function, and it may be considered as a wrapper around the back-end that adds
[memoisation](https://en.wikipedia.org/wiki/Memoization).
* `Delete(K)`: deletes the specified key from the cache.

The cache object is safe for concurrent access.

### Benchmarks

The following results have been achieved on Intel Core i5-8500T processor running Linux Mint 20.3
(with Go v1.17.6):

```
BenchmarkCache-6            	17759829	        64.92 ns/op
BenchmarkContendedCache-6   	  707421	      1505 ns/op
```

The benchmark is run by invoking `./test -b` from the root directory of the project. The script
generates and tests a cache with integer keys and values. The first benchmark reads the cache from
a single goroutine, while the second one is the same benchmark run in parallel with another 10 goroutines
accessing the cache concurrently.

### Status

Tested on Linux Mint 20.3 with Go version 1.17.6.
