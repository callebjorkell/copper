# Copper

![Copper](./copper.png)

_Copper keeps REST APIs honest_

## Introduction
Copper is a test library intended to verify that API contracts are upheld when exercising a REST API from a test. 
This is very useful for verifying that all paths and functionality that is declared in an API spec is in fact tested.

## Why?
Implementing these checks in tests will allow your application to check API correctness and backwards compatibility
before the code ever hits production. As long as the specification and application is not edited in the same change set,
breaking changes will also be prevented. Since the verification runs during builds only, there is no overhead during
production use, nor does Copper impose any sort of requirements on the implementation other than that it needs to 
conform to its own specification.

## Usage
Copper is used from integration style tests. Wrap the HTTP client being used in copper
and then use that client for performing the API calls in your test case. This works best from
a single main test for a single spec, and then subtest for the specific endpoints/use cases.

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
