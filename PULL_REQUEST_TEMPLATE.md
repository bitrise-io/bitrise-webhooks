### New Pull Request Checklist

- [ ] Run `go fmt` on your files (e.g. `go fmt ./service/common.go`, or on the whole `service` folder: `go fmt ./service/...`)
- [ ] Write tests for your code
  - The tests should cover both "success" and "error" cases.
  - The tests should also check **all the returned variables**, don't ignore any returned value!
  - Ideally the tests should be easily readable, we usually use tests to document our code instead of code comments.
    An example, if you'd write a comment like "Given X this function will return Y" or
    "Beware, if the input is X this function will return Y" then you should implement this as
    a unit test, instead of writing it as a comment.
- [ ] If your Pull Request is more than a bug fix you should also check `README.md` and change/add the descriptions there - also
  feel free to add yourself as a contributor if you implement support for a new service ;)
- [ ] Before creating the Pull Request you should also run `bitrise run test` with the [Bitrise CLI](https://www.bitrise.io/cli),
  to perform all the automatic checks (which will run on your Pull Request when you open it).


### Summary of Pull Request

This Pull Request makes the code 10x faster, while reducing
memory consumption by 99%, as well as it implements 5 new service support ...
Something like this ;)
