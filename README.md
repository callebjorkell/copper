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