# Idiomatic Go reference

The max-go house style: flat structs,
concrete types,
plain statements,
and interfaces only where there is real polymorphism.
Follow https://go.dev/doc/effective_go.
The examples below show the boring,
readable way to write the things that tempt people into overengineering.

## 1. Flat structs

Keep data shallow and searchable.
One level,
obvious fields.

```go
type Unit struct {
	ID      int
	Tag     string
	Team    int
	X, Y    int
	Hits    int
	MaxHits int
}
```

## 2. Plain loops, not closures

Write the statement.
A `for` loop is clearer than a chain of single use closures.

```go
func ReloadCost(units []Unit) int {
	total := 0
	for _, u := range units {
		total += u.MaxHits - u.Hits
	}
	return total
}
```

## 3. Concrete types; add interfaces only when a second one exists

A depot is one type with methods.
Introduce an interface at the *point of use* only if and when a second implementation actually appears.

```go
type Depot struct{}

func (Depot) Heal(u *Unit, n int)   { u.Hits = min(u.MaxHits, u.Hits+n) }
func (Depot) Repair(u *Unit, n int) { u.Hits = min(u.MaxHits, u.Hits+n) }
```

## 4. A `for` loop instead of generics + reflection

Type parametric machinery earns its place rarely.
Counting is not one of those times.

```go
func DamagedCount(units []Unit) int {
	n := 0
	for _, u := range units {
		if u.Hits < u.MaxHits {
			n++
		}
	}
	return n
}
```

## 5. Synchronous code for synchronous work

Goroutines and channels are for concurrency,
not for computing a value the caller just blocks on.

```go
func Add(a, b int) int { return a + b }
```

## Rules of thumb

- **Flat over nested** - shallow structs print, search, and read better.
- **Statement over closure** - don't build a function just to call it once.
- **Concrete over abstract** - accept interfaces, return structs; add an interface only with a real second implementation.
- **Loop over generics/reflection** - reach for type parameters only for genuine type parametric code.
- **Sync over async** - no goroutine for work the caller waits on anyway.
