# Go Learning Curriculum


**context.Context (no cancellation propagation)**
- [Go Blog: The context package](https://go.dev/blog/context)
- [Go Blog: Contexts and structs](https://go.dev/blog/context-and-structs)
- [context package docs](https://pkg.go.dev/context)

**Error wrapping (using fmt.Errorf but not %w)**
- [Go Blog: Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors)

## Tier 3 — Concurrency (go deep, it's the core of this project)

- [Go by Example: Mutexes](https://gobyexample.com/mutexes) — practical, shows exactly how to use sync.Mutex
- [Go Tour: Mutual Exclusion](https://go.dev/tour/concurrency/9) — short reinforcement, 5 mins
- [Data Race Detector](https://go.dev/doc/articles/race_detector) — read then run `go test -race ./...` on byte-space immediately

- [GopherCon 2017 — Understanding Channels](https://www.youtube.com/watch?v=KBZlN0izeiY) — channels vs mutexes, when to 

- [sync package docs](https://pkg.go.dev/sync) — keep open while adding locks

- [Go Tour: Mutual Exclusion](https://go.dev/tour/concurrency/9)
- [Go by Example: Mutexes](https://gobyexample.com/mutexes)
- [Data Race Detector](https://go.dev/doc/articles/race_detector) — run `go test -race ./...`
- [GopherCon 2018 — Rethinking Classical Concurrency Patterns](https://www.youtube.com/watch?v=5zXAHh5tJqQ)
- [GopherCon 2017 — Understanding Channels](https://www.youtube.com/watch?v=KBZlN0izeiY)
- [Go Memory Model](https://go.dev/ref/mem)
- [Go Blog: Pipelines and cancellation](https://go.dev/blog/pipelines)
- [Go Blog: Advanced Go Concurrency Patterns](https://go.dev/blog/advanced-go-concurrency-patterns)
- [sync package docs](https://pkg.go.dev/sync)
- [100 Go Mistakes — concurrency section](https://100go.co/)

---

## Tier 4 — Type system

**Nil interface gotcha**
- [Go FAQ: Why is nil not equal to nil](https://go.dev/doc/faq#nil_error)

**Generics**
- [Go Blog: An Introduction to Generics](https://go.dev/blog/intro-generics)
- [Go by Example: Generics](https://gobyexample.com/generics)

---

## Tier 5 — Professional engineering

**Testing & mocking**
- [Anthony GG — Go Testing](https://www.youtube.com/watch?v=d2geGE9fBHU)
- [Go Blog: Table Driven Tests](https://go.dev/wiki/TableDrivenTests)
- [Go by Example: Testing](https://gobyexample.com/testing)
- [Go testing package](https://pkg.go.dev/testing)
- [Add a test — official Go tutorial](https://go.dev/doc/tutorial/add-a-test)
- [afero testing docs](https://github.com/spf13/afero#testing-with-afero)

**Package design**
- [Go Blog: Organizing a Go module](https://go.dev/doc/modules/layout)
- [Dave Cheney: Avoid package names like base, util, or common](https://dave.cheney.net/2019/01/08/avoid-package-names-like-base-util-or-common)

**Profiling**
- [Go Blog: Profiling Go Programs](https://go.dev/blog/pprof)
