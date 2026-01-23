# GraphStructManager - Gremlin Query Builder [![Go Reference](https://pkg.go.dev/badge/github.com/jbrusegaard/graph-struct-manager.svg)](https://pkg.go.dev/github.com/jbrusegaard/graph-struct-manager)

A type-safe, chainable query builder for Gremlin graph databases in Go. This ORM provides an intuitive interface for building and executing Gremlin queries with full type safety.

## Table of Contents

- [Overview](#overview)
- [Setup](#setup)
  - [Custom Labels](#custom-labels)
- [Database Configuration](#database-configuration)
  - [Database Driver Types](#database-driver-types)
  - [Custom ID Generator](#custom-id-generator)
- [Hooks](#hooks)
- [Environment Variables](#environment-variables)
- [Query Builder Functions](#query-builder-functions)
  - [NewQuery](#newquery)
  - [Where](#where)
  - [WhereTraversal](#wheretraversal)
  - [AddSubTraversal](#addsubtraversal)
  - [Dedup](#dedup)
  - [Limit](#limit)
  - [Offset](#offset)
  - [OrderBy](#orderby)
  - [Find](#find)
  - [First](#first)
  - [Count](#count)
  - [Id](#id)
  - [Delete](#delete)
- [Complete Examples](#complete-examples)
- [Comparison Operators](#comparison-operators)

## Overview

The query builder uses Go generics to provide type-safe operations on vertex types that implement the `VertexType` interface. All functions are chainable, allowing for fluent query construction.

## Requirements

- Go 1.25+
- Gremlin 3.7.4

## Setup

First, define your vertex struct with the required gremlin tags shown below. By default, the vertex label will be your struct name converted to lower snake case. So for this example the created vertex label would be `test_vertex`.
The GSM expects that types.Vertex will be set as an anonymous struct on the struct in which you are creating a vertex.
```go
type TestVertex struct {
    types.Vertex                               // Anonymous embedding required
    Name        string   `gremlin:"name"`      // Field with gremlin tag
    Age         int      `gremlin:"age"`
    Email       string   `gremlin:"email"`
    Tags        []string `gremlin:"tags"`
}
```

### Capturing unmapped properties

Gremlin is schema-less, so results can include properties not represented in your struct. To preserve
them, add a map field tagged with `gremlin:"-,unmapped"`.

```go
type User struct {
    types.Vertex
    Name   string         `gremlin:"name"`
    Email  string         `gremlin:"email"`
    Extras map[string]any `gremlin:"-,unmapped"`
}
```

All properties returned by Gremlin that do not match a `gremlin` or `gremlinSubTraversal` tag will
be collected into the `Extras` map. You can mark multiple fields as `gremlin:"-,unmapped"`, but
each will receive a copy of the same data, which is memory-inefficient.

### Using omitempty

The `omitempty` option can be added to gremlin tags to skip fields with zero values when creating or updating vertices. This is similar to how JSON's `omitempty` works.

**Syntax:**
```go
type User struct {
    types.Vertex
    Name        string  `gremlin:"name"`                    // Always included
    Email       string  `gremlin:"email,omitempty"`         // Omit if empty string
    Age         int     `gremlin:"age,omitempty"`           // Omit if zero
    IsActive    bool    `gremlin:"is_active,omitempty"`     // Omit if false
    Tags        []string `gremlin:"tags,omitempty"`         // Omit if empty slice
    Metadata    *string  `gremlin:"metadata,omitempty"`     // Omit if nil
}
```

**When a field is omitted:**
- Empty strings (`""`)
- Zero numbers (`0`, `0.0`)
- False booleans (`false`)
- Nil pointers
- Empty slices, arrays, and maps
- Any type with a zero value

**When to use omitempty:**
- Optional fields that should not create properties in the graph when empty
- Reducing graph storage by not storing empty/default values
- When you want to distinguish between "not set" and "set to zero value"
- Fields that are populated conditionally

**Example:**
```go
// Create user with only non-empty fields
newUser := User{
    Name:  "John Doe",
    Email: "john@example.com",
    // Age is 0 (zero value) - will be omitted if has omitempty
    // IsActive is false (zero value) - will be omitted if has omitempty
    // Tags is nil - will be omitted if has omitempty
}

err := GSM.Create(db, &newUser)
// Only "name" and "email" properties will be created in the graph
// (assuming other fields have omitempty)
```

### Custom Labels

You can provide a custom label for your vertex by implementing the `Label()` method on your struct. This is useful when you need a specific label that differs from the normalized struct name, or when you want more control over the label format.

**Example with custom label:**
```go
type User struct {
    types.Vertex
    Name  string `gremlin:"name"`
    Email string `gremlin:"email"`
}

// Custom label implementation - supports both value and pointer receivers
func (u User) Label() string {
    return "custom_user_label"
}

// Or with pointer receiver:
// func (u *User) Label() string {
//     return "custom_user_label"
// }
```

**When to use custom labels:**
- When you need a specific label format that doesn't match the struct name pattern
- When migrating from existing graph databases with established label conventions
- When you want shorter or more descriptive labels than the auto-generated ones
- When working with multiple structs that should share the same label

**Default behavior:**
If you don't implement `Label()`, or if `Label()` returns an empty string, the system will automatically use the struct name normalized to snake_case (e.g., `MyCustomVertex` → `my_custom_vertex`). This ensures backward compatibility with existing code.

### Custom IDs

By default, the graph database automatically generates unique IDs for new vertices. However, you can provide a custom ID by setting the `ID` field in your struct before calling the `Create` function. This is useful when you need to maintain specific ID formats or integrate with existing systems.

**Example with custom ID:**
```go
type User struct {
    types.Vertex
    Name  string `gremlin:"name"`
    Email string `gremlin:"email"`
}

// Create user with custom ID
newUser := User{
    Name:  "John Doe",
    Email: "john@example.com",
}
newUser.ID = "custom-user-123" // Set custom ID

err := GSM.Create(db, &newUser)
if err != nil {
    log.Fatal(err)
}
// The vertex will be created with ID "custom-user-123"
```

**When to use custom IDs:**
- When integrating with external systems that have their own ID schemes
- When you need predictable or human-readable IDs
- When migrating data from other databases and need to preserve original IDs
- When implementing specific ID formats (e.g., UUIDs, prefixed IDs)

**Important notes:**
- If no ID is set, the database will automatically generate one
- Custom IDs must be unique within the graph
- The ID type can be string, int, or any type supported by your graph database

## Hooks

Implement hook interfaces on your vertex types to run logic before/after create or update.
Hooks receive the `*GremlinDriver` used for the operation and can abort by returning an error.

**Available hooks:**
- `BeforeCreate(db *GremlinDriver) error`
- `AfterCreate(db *GremlinDriver) error`
- `BeforeUpdate(db *GremlinDriver) error`
- `AfterUpdate(db *GremlinDriver) error`
- `AfterFind(db *GremlinDriver) error`

**Order of execution:**
- `Create` calls `BeforeCreate`, writes the vertex, sets `ID/CreatedAt/LastModified`, then `AfterCreate`.
- `Update` calls `BeforeUpdate`, writes the changes, updates `LastModified`, then `AfterUpdate`.
- `Save` dispatches to `Create` or `Update` based on whether `ID` is set.
- `Find`/`Take`/`ID` call `AfterFind` on each loaded vertex before returning.

**Example:**
```go
type User struct {
    types.Vertex
    Name   string `gremlin:"name"`
    Status string `gremlin:"status"`
}

func (u *User) BeforeCreate(db *driver.GremlinDriver) error {
    if u.Name == "" {
        return errors.New("name is required")
    }
    u.Status = "active"
    return nil
}

func (u *User) AfterCreate(db *driver.GremlinDriver) error {
    // e.g. enqueue an event
    return nil
}

func (u *User) BeforeUpdate(db *driver.GremlinDriver) error {
    if u.ID == nil {
        return errors.New("missing id")
    }
    return nil
}

func (u *User) AfterUpdate(db *driver.GremlinDriver) error {
    return nil
}
```

Import the necessary packages and connect to your Gremlin database:

```go
import (
    "github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
    "github.com/jbrusegaard/graph-struct-manager/comparator"
    // ... other imports
)

db, err := GSM.Open("ws://localhost:8182")
if err != nil {
    log.Fatal(err)
}
defer db.Close()
```

## Database Configuration

The `Open` function accepts an optional configuration parameter that allows you to customize the database driver behavior. You can specify the database driver type and provide a custom ID generator function.

### Configuration Options

```go
type Config struct {
    Driver      DatabaseDriver  // Database driver type (Gremlin or Neptune)
    IDGenerator func() any      // Custom ID generator function
}
```

### Database Driver Types

GraphStructManager supports multiple database backends:

- **`driver.Gremlin`** (default) - Standard Apache TinkerPop Gremlin Server
- **`driver.Neptune`** - AWS Neptune with Neptune-specific optimizations for handling slices and maps

**Example:**

```go
// Connect with default Gremlin driver
db, err := driver.Open("ws://localhost:8182")

// Connect with explicit Gremlin driver
db, err := driver.Open("ws://localhost:8182", driver.Config{
    Driver: driver.Gremlin,
})

// Connect to AWS Neptune
db, err := driver.Open("wss://your-neptune-endpoint:8182", driver.Config{
    Driver: driver.Neptune,
})
```

**When to specify the driver:**
- Use `driver.Gremlin` for standard TinkerPop Gremlin Server, JanusGraph, or other Gremlin-compatible databases
- Use `driver.Neptune` when connecting to AWS Neptune to enable Neptune-specific property handling for collections

### Custom ID Generator

By default, the graph database automatically generates unique IDs for new vertices. You can provide a custom ID generator function in the configuration to control how IDs are generated for all vertices created through the driver.

**Signature:**
```go
IDGenerator func() any
```

The function should return a unique identifier of any type supported by your graph database (string, int, UUID, etc.).

**Examples:**

```go
import (
    "github.com/google/uuid"
    "github.com/jbrusegaard/graph-struct-manager/gremlin/driver"
)

// Use UUID v4 for all new vertices
db, err := driver.Open("ws://localhost:8182", driver.Config{
    IDGenerator: func() any {
        return uuid.New().String()
    },
})

// Use custom prefixed IDs
var counter int64
db, err := driver.Open("ws://localhost:8182", driver.Config{
    IDGenerator: func() any {
        counter++
        return fmt.Sprintf("vertex-%d", counter)
    },
})

// Use timestamp-based IDs
db, err := driver.Open("ws://localhost:8182", driver.Config{
    IDGenerator: func() any {
        return time.Now().UnixNano()
    },
})

// Combine driver type and ID generator
db, err := driver.Open("wss://neptune-endpoint:8182", driver.Config{
    Driver: driver.Neptune,
    IDGenerator: func() any {
        return uuid.New().String()
    },
})
```

**When to use a custom ID generator:**
- When you need consistent ID formats across all vertices (e.g., all UUIDs)
- When integrating with external systems that expect specific ID schemes
- When you want readable or predictable IDs for debugging
- When implementing distributed systems that require globally unique IDs

**Important notes:**
- The ID generator is called for every `Create` operation
- The function must return unique values to avoid conflicts
- If `IDGenerator` is `nil` (default), the database will auto-generate IDs
- The generator function should be thread-safe if used in concurrent environments
- Individual vertices can still override the ID by setting the `ID` field before calling `Create` (see [Custom IDs](#custom-ids) section)

## Environment Variables

GraphStructManager supports the following environment variables for configuration and debugging:

### GSM_LOG_LEVEL

Controls the logging level for the library. Available values:
- `debug` - Most verbose logging
- `info` - Standard informational logging (default)
- `warn` - Warning messages only
- `error` - Error messages only
- `fatal` - Fatal errors only

**Example:**
```bash
export GSM_LOG_LEVEL=debug
```

### GSM_DEBUG

When set to `true`, enables query debugging which logs the generated Gremlin query strings before execution. This is useful for troubleshooting and understanding what queries are being sent to the database.

**Example:**
```bash
export GSM_DEBUG=true
```

**Output example:**
```
INFO Running Query: V().HasLabel('test_vertex').Has('name', 'John').Limit(1).Next()
```

## Query Builder Functions

### NewQuery[T]

Creates a new query builder for the specified vertex type.

**Signature:**
```go
func NewQuery[T VertexType](db *GremlinDriver) *Query[T]
```

**Usage:**
```go
// Create a new query builder for TestVertex
query := GSM.NewQuery[TestVertex](db)

// Or use the convenience function
query := GSM.Model[TestVertex](db)
```

### Where

Adds a condition to the query using comparison operators.

**Signature:**
```go
func (q *Query[T]) Where(field string, operator comparator.Comparator, value any) *Query[T]
```

**Examples:**
```go
// Equal comparison
users := GSM.Model[TestVertex](db).Where("name", comparator.EQ, "John")

// Not equal
users := GSM.Model[TestVertex](db).Where("age", comparator.NEQ, 25)

// Greater than
users := GSM.Model[TestVertex](db).Where("age", comparator.GT, 18)

// Greater than or equal
users := GSM.Model[TestVertex](db).Where("age", comparator.GTE, 21)

// Less than
users := GSM.Model[TestVertex](db).Where("age", comparator.LT, 65)

// Less than or equal
users := GSM.Model[TestVertex](db).Where("age", comparator.LTE, 30)

// In array
users := GSM.Model[TestVertex](db).Where("name", comparator.IN, []any{"John", "Jane", "Bob"})

// Contains (for string fields)
users := GSM.Model[TestVertex](db).Where("email", comparator.CONTAINS, "@gmail.com")

// Without (exclude values from array)
users := GSM.Model[TestVertex](db).Where("status", comparator.WITHOUT, []any{"banned", "suspended"})

// Chain multiple conditions
users := GSM.Model[TestVertex](db).
    Where("age", comparator.GT, 18).
    Where("email", comparator.CONTAINS, "@company.com")
```

### WhereTraversal

Adds a custom Gremlin traversal condition for advanced queries.

**Signature:**
```go
func (q *Query[T]) WhereTraversal(traversal *gremlingo.GraphTraversal) *Query[T]
```

**Examples:**
```go
// Custom traversal with has step
users := GSM.Model[TestVertex](db).
    WhereTraversal(gremlingo.T__.Has("name", "John"))

// Complex traversal
users := GSM.Model[TestVertex](db).
    WhereTraversal(gremlingo.T__.Has("age", gremlingo.P.Between(25, 35)))

// Combine with regular Where conditions
users := GSM.Model[TestVertex](db).
    Where("name", comparator.EQ, "John").
    WhereTraversal(gremlingo.T__.Has("email", gremlingo.P.StartingWith("j")))
```

### AddSubTraversal

Allows you to pass sub traversals that will be executed and mapped to struct fields based on their gremlin tags. This is useful when you need to fetch related data or perform complex traversals that should populate specific fields in your struct.

**Signature:**
```go
func (q *Query[T]) AddSubTraversal(gremlinTag string, traversal *gremlingo.GraphTraversal) *Query[T]
```

**How it works:**
- The `gremlinTag` parameter must match a `gremlin` tag on a field in your struct
- The `traversal` is executed as part of the query and its result is projected
- The result from the subtraversal is automatically mapped to the struct field with the matching gremlin tag

**Examples:**

```go
// Define a struct with a field that will be populated by a subtraversal
type User struct {
    types.Vertex
    Name        string   `gremlin:"name"`
    Email       string   `gremlin:"email"`
    FriendCount int      `gremlinSubTraversal:"friend_count"`  // Will be populated by subtraversal
    Friends     []string `gremlinSubTraversal:"friends"`        // Another subtraversal field
}

// Get user with friend count using a subtraversal
user, err := GSM.Model[User](db).
    Where("email", comparator.EQ, "john@example.com").
    AddSubTraversal("friend_count", gremlingo.T__.Out("friends").Count()).
    First()

// Get user with list of friend names
user, err := GSM.Model[User](db).
    Where("email", comparator.EQ, "john@example.com").
    AddSubTraversal("friends", gremlingo.T__.Out("friends").Values("name").Fold()).
    First()

// Multiple subtraversals for different fields
user, err := GSM.Model[User](db).
    Where("email", comparator.EQ, "john@example.com").
    AddSubTraversal("friend_count", gremlingo.T__.Out("friends").Count()).
    AddSubTraversal("friends", gremlingo.T__.Out("friends").Values("name").Fold()).
    First()

// Complex subtraversal - get average age of friends
type UserWithStats struct {
    types.Vertex
    Name           string  `gremlin:"name"`
    AvgFriendAge   float64 `gremlinSubTraversal:"avg_friend_age"`  // Populated by subtraversal
}

user, err := GSM.Model[UserWithStats](db).
    Where("name", comparator.EQ, "John").
    AddSubTraversal("avg_friend_age",
        gremlingo.T__.Out("friends").
            Values("age").
            Mean()).
    First()
```

**Important notes:**
- The gremlin tag in `AddSubTraversal` must exactly match the `gremlinSubTraversal` tag on the struct field
- Subtraversals are executed as part of the main query using Gremlin's `Project` step
- The result type from the subtraversal must be compatible with the struct field type
- You can add multiple subtraversals to populate different fields in a single query
- Subtraversals work with `Find()`, `First()`, and other query execution methods

### Dedup

Removes duplicate results from the query.

**Signature:**
```go
func (q *Query[T]) Dedup() *Query[T]
```

**Examples:**
```go
// Remove duplicates
uniqueUsers := GSM.Model[TestVertex](db).
    Where("tags", comparator.CONTAINS, "developer").
    Dedup()

// Chain with other operations
users := GSM.Model[TestVertex](db).
    Where("age", comparator.GT, 25).
    Dedup().
    OrderBy("name", driver.Asc)
```

### Limit

Sets the maximum number of results to return.

**Signature:**
```go
func (q *Query[T]) Limit(limit int) *Query[T]
```

**Examples:**
```go
// Get first 10 users
users := GSM.Model[TestVertex](db).
    OrderBy("name", driver.Asc).
    Limit(10)

// Top 5 oldest users
oldestUsers := GSM.Model[TestVertex](db).
    OrderBy("age", driver.Desc).
    Limit(5)

// Combine with where conditions
activeUsers := GSM.Model[TestVertex](db).
    Where("status", comparator.EQ, "active").
    Limit(20)
```

### Offset

Sets the number of results to skip (for pagination).

**Signature:**
```go
func (q *Query[T]) Offset(offset int) *Query[T]
```

**Examples:**
```go
// Skip first 20 results (page 2 with 20 per page)
users := GSM.Model[TestVertex](db).
    OrderBy("name", driver.Asc).
    Offset(20).
    Limit(20)

// Get results 50-100
users := GSM.Model[TestVertex](db).
    Offset(50).
    Limit(50)

// Pagination helper function
func getPage(db *GSM.GremlinDriver, page, pageSize int) ([]TestVertex, error) {
    return GSM.Model[TestVertex](db).
        OrderBy("id", driver.Asc).
        Offset((page - 1) * pageSize).
        Limit(pageSize).
        Find()
}
```

### Range

Sets a range of results to return using Gremlin's native `range()` step. This provides an alternative to using `Offset()` and `Limit()` together.

**Signature:**
```go
func (q *Query[T]) Range(lower int, upper int) *Query[T]
```

**Important Notes:**
- **Range cannot be used with Offset** - Using both together will cause undefined behavior. If `Offset()` is set, `Range()` will be ignored.
- The range is **inclusive of the lower bound** and **exclusive of the upper bound** (similar to Go slices)
- Range bounds are zero-indexed

**Examples:**
```go
// Get results 0-9 (first 10 results)
users := GSM.Model[TestVertex](db).
    OrderBy("name", driver.Asc).
    Range(0, 10)

// Get results 10-19 (second page of 10)
users := GSM.Model[TestVertex](db).
    OrderBy("name", driver.Asc).
    Range(10, 20)

// Get results 50-99
users := GSM.Model[TestVertex](db).
    Range(50, 100)

// Pagination using Range
func getPageWithRange(db *GSM.GremlinDriver, page, pageSize int) ([]TestVertex, error) {
    lower := (page - 1) * pageSize
    upper := lower + pageSize
    return GSM.Model[TestVertex](db).
        OrderBy("id", driver.Asc).
        Range(lower, upper).
        Find()
}

// INCORRECT - Don't use Range with Offset (will be ignored)
users := GSM.Model[TestVertex](db).
    Offset(10).    // This will cause Range to be ignored
    Range(0, 10).  // This will be ignored!
    Find()
```

### OrderBy

Adds ordering to the query with ascending or descending direction.

**Signature:**
```go
func (q *Query[T]) OrderBy(field string, order GremlinOrder) *Query[T]
```

**Order Constants:**
- `driver.Asc` - Ascending order
- `driver.Desc` - Descending order

**Examples:**
```go
// Order by name (ascending)
users := GSM.Model[TestVertex](db).
    OrderBy("name", driver.Asc)

// Order by age (descending)
users := GSM.Model[TestVertex](db).
    OrderBy("age", driver.Desc)

// Combine with filtering
youngUsers := GSM.Model[TestVertex](db).
    Where("age", comparator.LT, 30).
    OrderBy("age", driver.Asc)
```


### Find

Executes the query and returns all matching results.

**Signature:**
```go
func (q *Query[T]) Find() ([]T, error)
```

**Examples:**
```go
// Get all users
allUsers, err := GSM.Model[TestVertex](db).Find()
if err != nil {
    return err
}

// Get filtered results
activeUsers, err := GSM.Model[TestVertex](db).
    Where("status", comparator.EQ, "active").
    Find()

// Get paginated results
users, err := GSM.Model[TestVertex](db).
    OrderBy("name", driver.Asc).
    Limit(50).
    Find()

// Complex query
developers, err := GSM.Model[TestVertex](db).
    Where("department", comparator.EQ, "engineering").
    Where("experience", comparator.GTE, 2).
    OrderBy("salary", driver.Desc).
    Find()
```

### First

Executes the query and returns the first result.

**Signature:**
```go
func (q *Query[T]) First() (T, error)
```

**Examples:**
```go
// Get first user by name
user, err := GSM.Model[TestVertex](db).
    Where("name", comparator.EQ, "John").
    First()
if err != nil {
    return err
}

// Get oldest user
oldestUser, err := GSM.Model[TestVertex](db).
    OrderBy("age", driver.Desc).
    First()

// Get user with specific email
user, err := GSM.Model[TestVertex](db).
    Where("email", comparator.EQ, "john@example.com").
    First()

// Handle not found
user, err := GSM.Model[TestVertex](db).
    Where("id", comparator.EQ, nonExistentId).
    First()
if err != nil {
    if err.Error() == "no more results" {
        // Handle not found case
        fmt.Println("User not found")
    } else {
        // Handle other errors
        return err
    }
}
```

### Count

Returns the number of matching results without retrieving the actual data.

**Signature:**
```go
func (q *Query[T]) Count() (int, error)
```

**Examples:**
```go
// Count all users
totalUsers, err := GSM.Model[TestVertex](db).Count()
if err != nil {
    return err
}

// Count active users
activeCount, err := GSM.Model[TestVertex](db).
    Where("status", comparator.EQ, "active").
    Count()

// Count users in age range
adultsCount, err := GSM.Model[TestVertex](db).
    Where("age", comparator.GTE, 18).
    Where("age", comparator.LTE, 65).
    Count()

// Check if any users exist with condition
hasAdmins, err := GSM.Model[TestVertex](db).
    Where("role", comparator.EQ, "admin").
    Count()
if err != nil {
    return err
}
if hasAdmins > 0 {
    fmt.Println("Admin users exist")
}
```

### Id

Finds a vertex by its ID using direct graph index lookup for optimal performance.

**Signature:**
```go
func (q *Query[T]) Id(id any) (T, error)
```

**Examples:**
```go
// Find user by ID (most efficient lookup)
user, err := GSM.Model[TestVertex](db).Id("user-123")
if err != nil {
    return err
}

// Find vertex by numeric ID
vertex, err := GSM.Model[TestVertex](db).Id(12345)
if err != nil {
    if err.Error() == "no more results" {
        fmt.Println("Vertex not found")
    } else {
        return err
    }
}

// Using with UUID
import "github.com/google/uuid"
userID := uuid.New()
user, err := GSM.Model[TestVertex](db).Id(userID)
```

### Delete

Deletes all vertices matching the query conditions.

**Signature:**
```go
func (q *Query[T]) Delete() error
```

**Examples:**
```go
// Delete specific user
err := GSM.Model[TestVertex](db).
    Where("email", comparator.EQ, "user@example.com").
    Delete()

// Delete inactive users
err := GSM.Model[TestVertex](db).
    Where("status", comparator.EQ, "inactive").
    Delete()

// Delete users older than 100 (cleanup)
err := GSM.Model[TestVertex](db).
    Where("age", comparator.GT, 100).
    Delete()

// Delete with multiple conditions
err := GSM.Model[TestVertex](db).
    Where("department", comparator.EQ, "temp").
    Where("lastLogin", comparator.LT, oneYearAgo).
    Delete()

// Delete users excluding certain roles
err := GSM.Model[TestVertex](db).
    Where("role", comparator.WITHOUT, []any{"admin", "super_admin"}).
    Where("lastLogin", comparator.LT, sixMonthsAgo).
    Delete()

if err != nil {
    log.Printf("Failed to delete users: %v", err)
    return err
}
```

## Complete Examples

### Basic CRUD Operations

```go
func main() {
    // Setup
    db, err := GSM.Open("ws://localhost:8182")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create a user
    newUser := TestVertex{
        Name:  "Alice Johnson",
        Age:   28,
        Email: "alice@example.com",
        Tags:  []string{"developer", "golang", "senior"},
    }

    err = GSM.Create(db, &newUser)
    if err != nil {
        log.Fatal(err)
    }

    // Read - Find user by email
    user, err := GSM.Model[TestVertex](db).
        Where("email", comparator.EQ, "alice@example.com").
        First()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Found user: %+v\n", user)

    // Read by ID (fastest lookup method)
    userByID, err := GSM.Model[TestVertex](db).Id(newUser.Id)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Found user by ID: %+v\n", userByID)

    // Update would typically involve Create with existing ID

    // Delete - Remove user
    err = GSM.Model[TestVertex](db).
        Where("email", comparator.EQ, "alice@example.com").
        Delete()
    if err != nil {
        log.Fatal(err)
    }
}
```

### Advanced Querying

```go
func advancedQueries(db *GSM.GremlinDriver) {
    // Pagination
    page := 2
    pageSize := 10
    users, err := GSM.Model[TestVertex](db).
        OrderBy("name", driver.Asc).
        Offset((page-1) * pageSize).
        Limit(pageSize).
        Find()

    // Search with multiple filters
    seniorDevelopers, err := GSM.Model[TestVertex](db).
        Where("age", comparator.GTE, 25).
        Where("experience", comparator.GT, 3).
        Where("tags", comparator.CONTAINS, "senior").
        OrderBy("experience", driver.Desc).
        Find()

    // Find active users excluding certain statuses
    activeUsers, err := GSM.Model[TestVertex](db).
        Where("status", comparator.WITHOUT, []any{"banned", "suspended", "deleted"}).
        Where("lastLogin", comparator.GTE, thirtyDaysAgo).
        Find()

    // Count and statistics
    totalDevelopers, err := GSM.Model[TestVertex](db).
        Where("tags", comparator.CONTAINS, "developer").
        Count()

    juniorCount, err := GSM.Model[TestVertex](db).
        Where("tags", comparator.CONTAINS, "junior").
        Count()

    fmt.Printf("Total developers: %d, Junior: %d\n", totalDevelopers, juniorCount)

    // Complex query with custom traversal
    complexResults, err := GSM.Model[TestVertex](db).
        Where("department", comparator.EQ, "engineering").
        WhereTraversal(gremlingo.T__.Has("salary", gremlingo.P.Between(50000, 100000))).
        OrderBy("lastModified", driver.Desc).
        Limit(20).
        Find()
}
```

### Error Handling Patterns

```go
func handleQueryErrors(db *GSM.GremlinDriver) {
    // Handle "not found" gracefully
    user, err := GSM.Model[TestVertex](db).
        Where("id", comparator.EQ, "non-existent-id").
        First()

    if err != nil {
        if strings.Contains(err.Error(), "no more results") {
            fmt.Println("User not found")
            // Handle not found case
            return
        }
        // Handle other errors
        log.Printf("Query error: %v", err)
        return
    }

    // Check if results exist before processing
    count, err := GSM.Model[TestVertex](db).
        Where("status", comparator.EQ, "pending").
        Count()

    if err != nil {
        log.Printf("Count error: %v", err)
        return
    }

    if count == 0 {
        fmt.Println("No pending users found")
        return
    }

    // Process pending users
    pendingUsers, err := GSM.Model[TestVertex](db).
        Where("status", comparator.EQ, "pending").
        Find()
    // ... process users
}
```

## Comparison Operators

The following comparison operators are available in the `comparator` package:

| Operator | Constant | Description | Example |
|----------|----------|-------------|---------|
| `=` | `comparator.EQ` | Equal to | `Where("age", comparator.EQ, 25)` |
| `!=` | `comparator.NEQ` | Not equal to | `Where("status", comparator.NEQ, "inactive")` |
| `>` | `comparator.GT` | Greater than | `Where("age", comparator.GT, 18)` |
| `>=` | `comparator.GTE` | Greater than or equal | `Where("score", comparator.GTE, 80)` |
| `<` | `comparator.LT` | Less than | `Where("age", comparator.LT, 65)` |
| `<=` | `comparator.LTE` | Less than or equal | `Where("attempts", comparator.LTE, 3)` |
| `in` | `comparator.IN` | Value in array | `Where("role", comparator.IN, []any{"admin", "user"})` |
| `contains` | `comparator.CONTAINS` | String contains | `Where("email", comparator.CONTAINS, "@gmail.com")` |
| `without` | `comparator.WITHOUT` | Exclude values from array | `Where("status", comparator.WITHOUT, []any{"banned", "suspended"})` |

## Performance Tips

1. **Use Id() for direct lookups** when you know the vertex ID - this hits the graph index directly and is the fastest lookup method
2. **Use Count() for existence checks** instead of Find() when you only need to know if records exist
3. **Apply filters early** in the chain to reduce the dataset size
4. **Use Limit()** for large result sets to prevent memory issues
5. **Order results** consistently when using Offset() for pagination
6. **Consider using indices** on frequently queried fields in your Gremlin database

## Thread Safety

The query builder creates a new query instance for each operation and is safe to use concurrently. However, the underlying database connection should be managed appropriately for concurrent access.
