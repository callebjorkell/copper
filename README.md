# Copper

![Copper](./copper_small.png)

_Copper keeps REST APIs honest_

# Introduction
Copper is a test library intended to verify that API contracts are upheld when exercising a REST API from a test. Copper
will make sure that all the endpoints in your [OpenAPI](https://www.openapis.org/) specification are tested, and that
bodies, paths, parameters, and response codes exchanged between the test client and server (system under test) are 
correct. Copper will also fail the test if undocumented endpoints are being visited.

# Why?
API contracts and documentation have always been a good way to communicate intent between a backend API and client code.
However, throughout the life-time of a project, things happen:
- Endpoints change body payloads, parameters, or responses. 
- Endpoints are added, but documentation is forgotten.
- OpenAPI specification becomes invalid (was it ever actually valid?)
- Test coverage is missing.

All of the above points contribute to untrustworthy specifications, which in turn seriously reduces their effectiveness
as a tool of communication and reference.

Copper resolves all of these pain points in a non-intrusive way by verifying interactions in the test phase. This
means the APIs can stay lean during operation, while confidence in both documentation and implementation can remain
high.

## 100% test coverage
Copper enforces 100% test coverage of all declared paths, methods, and response codes. This does not mean that the
application will have a 100% line coverage, which in general is seen as overkill and both impractical and having low
return on investment. Having a 100% coverage of API endpoints, however, allows for the important main paths of the API
to be verified in tests, and Copper provides an automated check for that this is indeed happening.

Having full test coverage of the API also helps retain backwards compatibility of changes by creating a circular
dependency between the tests and the specification. This also makes sure that any new endpoints have to be both 
documented and tested, or neither.

NOTE: This behaviour can be disabled with an [option](#options).

# Usage
Copper is used from integration/contract style tests. Wrap the HTTP client being used in copper and then use that client 
for performing the API calls in your test case. This works best from a single main test for a single spec, and then 
subtest for the specific endpoints/use cases.

```go
func TestVersion1(t *testing.T) {
	f, err := os.Open("spec.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	client, err := copper.WrapClient(http.DefaultClient, f)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("ping", func(t *testing.T) {
		testPingEndpoints(t, client)
	}
	t.Run("other use case that I have", func(t *testing.T) {
		testMyOtherThing(t, client)
	}
	
	// Verifying at the end checks that all paths, methods and responses are covered and that no extra paths have been hit.
	client.Verify(t)
}
```
See the [examples](examples) for complete examples.

## Options
To alter the behavior of copper and control what type of validation will be done, functional options can be passed to
the `WrapClient` or stand-alone `NewVerifier` constructors. The options are as follows:
- `WithServer`: Sets the server and base path for the API to allow the spec to be mapped to the actual server endpoints.
This is especially useful if the spec doesn't contain a server with the base path that the target of the tests have, or
if the wrong entry is being used for verification. Without this option, the first server in the list that matches the
base path of the request will be used.
- `WithInternalServerErrors`: Also verify that all declared 500 responses have been tested. This is not really
recommended since if an internal server error can be produced in a test, the problem should probably just be fixed 
instead.
- `WithRequestValidation`: Also validate that the request adheres to the spec. This can be useful when developing the
tests as it checks that the client is well-behaved, but makes less sense once the contract tests are done, as [the server
should ideally be lenient in the data that it accepts](https://en.wikipedia.org/wiki/Robustness_principle).
- `WithoutFullCoverage`: Do not require full coverage of all methods, paths and response codes. 

# Building
As Copper is a library, it will not build into a standalone binary. Copper is a standard go project, and only needs
the go tooling to test:
```shell
go vet ./... 
go test ./...
```
