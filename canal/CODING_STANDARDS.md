# Elixir/Phoenix Coding Standards

These standards apply to Elixir/Phoenix projects. LLM agents and human contributors must follow these conventions.

## Tooling is Authoritative

The following tools define and enforce our coding standards:

| Tool | Command | Purpose |
|------|---------|---------|
| **mix format** | `mix format` | Code formatting (line length, spacing, indentation) |
| **Credo** | `mix credo --strict` | Code quality, consistency, readability |
| **Dialyzer** | `mix dialyzer` | Type checking via typespecs |

**Rules:**
- Run `mix format` before committing
- All code must pass `mix credo --strict` with no errors
- All code must pass `mix dialyzer` with no warnings
- Treat warnings as errors during compilation (`--warnings-as-errors`)

**Line length:** The formatter targets 98 characters (default). Credo allows up to 120 characters but flags longer lines as low-priority warnings. Aim for 98; occasional lines up to 120 are acceptable.

If a tool enforces a rule, that rule is not documented here. If you're unsure about formatting or style, run the tools and follow their output.

---

## Module Structure

Organize modules in this order:

```elixir
defmodule MyApp.Context.Entity do
  @moduledoc """
  Brief description of what this module does.
  """

  # 1. use/import/alias/require (in this order)
  use Ecto.Schema
  import Ecto.Changeset
  import Ecto.Query
  alias MyApp.Repo
  alias MyApp.Context.{OtherEntity, AnotherEntity}

  # 2. Module attributes
  @primary_key {:id, :integer, autogenerate: false}
  @topic "entity"

  # 3. Schema (if applicable)
  schema "entities" do
    field :name, :string
    belongs_to :parent, Parent
  end

  # 4. Public functions with @doc and @spec
  @doc """
  Returns all entities.
  """
  @spec list_entities() :: [Entity.t()]
  def list_entities do
    Repo.all(Entity)
  end

  # 5. Private functions
  defp helper_function(arg) do
    # ...
  end
end
```

---

## Documentation

### Module Documentation

Public API modules (contexts, controllers, LiveViews) should have a `@moduledoc`:

```elixir
@moduledoc """
The Species context.

Provides functions for querying and managing species records,
including their relationships to galls, hosts, and images.
"""
```

For internal/infrastructure modules (Application, Repo, generated code), use `@moduledoc false`.

For schema modules, briefly describe what the entity represents.

### Function Documentation

Public functions must have `@doc` and `@spec`:

```elixir
@doc """
Fetches a species by ID.

Returns `nil` if not found.
"""
@spec get_species(integer()) :: Species.t() | nil
def get_species(id), do: Repo.get(Species, id)
```

Private functions do not need `@doc` or `@spec` unless complex.

### Typespecs

Use typespecs for:
- All public function arguments and return values
- Custom types that improve readability
- Structs (define `t()` type)

```elixir
@type t :: %__MODULE__{
  id: integer(),
  name: String.t(),
  description: String.t() | nil,      # Optional field (nullable in DB)
  taxonomy: Taxonomy.t() | nil,       # Association (nil if not preloaded)
  inserted_at: DateTime.t()
}
```

Use `| nil` for fields that can be null in the database or associations that may not be loaded.

---

## Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Modules | PascalCase | `GallformersWeb.SpeciesLive` |
| Functions | snake_case | `get_species_by_name/1` |
| Variables | snake_case | `species_list` |
| Atoms | snake_case | `:species_created` |
| Files | snake_case | `species_live.ex` |
| LiveView modules | `*Live` suffix | `SpeciesLive`, `HostsLive.Index` |
| Context modules | Plural noun | `Species`, `Hosts`, `Accounts` |
| Schema modules | Singular noun | `Species.Gall`, `Hosts.Host` |

**Predicate functions** end with `?`:
```elixir
def valid?(changeset), do: changeset.valid?
```

**Never** prefix predicates with `is_`:
```elixir
# WRONG
def is_valid(changeset)

# CORRECT
def valid?(changeset)
```

---

## Code Organization

### Phoenix Contexts

Organize business logic into contexts (domain modules):

```
lib/my_app/
├── species.ex         # Species context (public API)
├── species/
│   ├── species.ex     # Species schema
│   ├── gall.ex        # Gall schema
│   └── image.ex       # Image schema
├── hosts.ex           # Hosts context
└── hosts/
    └── host.ex        # Host schema
```

**Context modules** expose the public API. **Schema modules** define data structures.

### Keep Related Code Together

- One module per file
- Files mirror module namespace (`MyApp.Species.Gall` → `lib/my_app/species/gall.ex`)
- Tests mirror source structure (`lib/my_app/species.ex` → `test/my_app/species_test.exs`)

---

## Ecto Patterns

### Queries

Use Ecto's query syntax, not raw SQL:

```elixir
def list_species_by_family(family_id) do
  from(s in Species,
    join: t in assoc(s, :taxonomy),
    where: t.family_id == ^family_id,
    order_by: [asc: s.name],
    preload: [:taxonomy, :images]
  )
  |> Repo.all()
end
```

### Changesets

Define changesets in schema modules:

```elixir
def changeset(species, attrs) do
  species
  |> cast(attrs, [:name, :description, :abundance_id])
  |> validate_required([:name])
  |> unique_constraint(:name)
end
```

**Never** put user-input fields and programmatic fields in the same `cast/3`:

```elixir
# User input
|> cast(attrs, [:name, :description])
# Then set programmatic fields explicitly
|> put_change(:updated_by, user_id)
```

### Preloading

Always preload associations that will be accessed:

```elixir
# In context
species = Repo.get(Species, id) |> Repo.preload([:taxonomy, :images])

# Or in query
from(s in Species, preload: [:taxonomy, :images])
```

### Avoiding N+1 Queries

N+1 occurs when you query a list, then query each item's association separately:

```elixir
# BAD - N+1 (1 query for species, N queries for taxonomy)
species = Repo.all(Species)
Enum.map(species, fn s -> s.taxonomy.name end)  # Each access triggers a query

# GOOD - Preload in the original query
species = Repo.all(from s in Species, preload: [:taxonomy])
Enum.map(species, fn s -> s.taxonomy.name end)  # No additional queries
```

**Detection:** Enable query logging in dev to spot repeated queries:
```elixir
# config/dev.exs
config :my_app, MyApp.Repo, log: :debug
```

### Query Performance

- Use `select` to fetch only needed fields for large result sets
- Add database indexes for frequently filtered/joined columns
- Use `Repo.stream/1` for processing large datasets without loading all into memory

---

## Error Handling

### Pattern Match Results

Handle success and error cases explicitly:

```elixir
case Species.create_species(attrs) do
  {:ok, species} ->
    {:noreply,
     socket
     |> put_flash(:info, "Species created")
     |> push_navigate(to: ~p"/species/#{species}")}

  {:error, changeset} ->
    {:noreply, assign(socket, form: to_form(changeset))}
end
```

### Use `with` for Multi-Step Operations

Use `with` when chaining 2+ operations that can fail. For single checks, prefer `case`.

```elixir
with %Species{} = species <- Repo.get(Species, id),
     :ok <- authorize(user, :update, species),
     {:ok, updated} <- Species.update_species(species, attrs) do
  {:ok, updated}
else
  nil -> {:error, :not_found}
  {:error, reason} -> {:error, reason}
end
```

**Note**: Match on actual return types. `Repo.get/2` returns `struct | nil`, not `{:ok, struct}`.

---

## Testing

Examples use generic `MyApp` module names. Replace with your project's module prefix.

### Test File Structure

```elixir
defmodule MyApp.SpeciesTest do
  use MyApp.DataCase

  alias MyApp.Species

  describe "list_species/0" do
    test "returns all species" do
      species = species_fixture()
      assert Species.list_species() == [species]
    end
  end
end
```

### Fixtures

Test fixtures are defined in `test/support/fixtures/` and imported via `DataCase` or `ConnCase`:

```elixir
defmodule MyApp.SpeciesFixtures do
  def species_fixture(attrs \\ %{}) do
    {:ok, species} =
      attrs
      |> Enum.into(%{name: "Test Species"})
      |> MyApp.Species.create_species()

    species
  end
end
```

Import in tests: `import MyApp.SpeciesFixtures`

### Assertions

- Use `assert` and `refute`, not `assert x == true`
- Test behavior, not implementation
- Use fixtures for test data (define in `test/support/fixtures/`)

---

## Logging

Use the `Logger` module with appropriate levels:

```elixir
require Logger

Logger.debug("Query executed", sql: query, params: params)  # Development details
Logger.info("User signed in", user_id: user.id)             # Significant events
Logger.warning("Rate limit approaching", count: count)      # Unexpected but recoverable
Logger.error("Payment failed", error: reason, order_id: id) # Failures requiring attention
```

**Guidelines:**
- Use structured metadata (keyword lists) over string interpolation
- Never log PII (emails, passwords, tokens, full names)
- Use `debug` for high-volume or detailed tracing
- Use `info` for business events (user actions, state changes)
- Use `warning` for recoverable issues
- Use `error` for failures that need investigation

---

## OTP Patterns

For background work, prefer the simplest tool that fits:

| Need | Use | Example |
|------|-----|---------|
| One-off async work | `Task.async/1` or `Task.Supervisor` | Sending emails |
| Periodic work | `Process.send_after/3` in GenServer | Cache expiration |
| Stateful process | `GenServer` | Connection pool, rate limiter |
| Simple state | `Agent` | Counters, simple caches |

**GenServer basics:**

```elixir
defmodule MyApp.Counter do
  use GenServer

  # Client API
  def start_link(opts), do: GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  def increment, do: GenServer.call(__MODULE__, :increment)

  # Server callbacks
  @impl true
  def init(_opts), do: {:ok, 0}

  @impl true
  def handle_call(:increment, _from, count), do: {:reply, count + 1, count + 1}
end
```

**Notes:**
- Always add GenServers to a supervision tree
- Use `@impl true` for callback functions
- Prefer named processes (`name: __MODULE__`) for singletons
- See [Elixir GenServer docs](https://hexdocs.pm/elixir/GenServer.html) for advanced patterns

---

## Security

Phoenix provides strong defaults. Don't disable them without understanding the risks.

### CSRF Protection

Phoenix forms include CSRF tokens automatically. Never:
- Disable `Plug.CSRFProtection` in router
- Use `raw` to bypass token insertion
- Accept form data without `phx-submit` or standard form POST

### Input Handling

```elixir
# SAFE - Ecto parameterizes values
from(s in Species, where: s.name == ^user_input)

# DANGEROUS - SQL injection risk
from(s in Species, where: fragment("name = '#{user_input}'"))

# SAFE - Use parameterized fragments
from(s in Species, where: fragment("lower(?) LIKE ?", s.name, ^pattern))
```

### HTML Output

```heex
<%!-- SAFE - Phoenix escapes by default --%>
<p>{@user_comment}</p>

<%!-- DANGEROUS - Only use for trusted HTML (admin-generated markdown, etc.) --%>
<p>{raw(@trusted_html)}</p>
```

### Atom Creation

```elixir
# DANGEROUS - Atoms are never garbage collected
String.to_atom(user_input)

# SAFE - Only creates existing atoms
String.to_existing_atom(user_input)
```

---

## Configuration

Phoenix uses multiple config files for different purposes:

| File | When Evaluated | Use For |
|------|----------------|---------|
| `config/config.exs` | Compile time | Shared settings, imported by all envs |
| `config/dev.exs` | Compile time | Dev-only settings (debug, local URLs) |
| `config/test.exs` | Compile time | Test settings (async, test DB) |
| `config/prod.exs` | Compile time | Production defaults (not secrets) |
| `config/runtime.exs` | Runtime | **Secrets**, env vars, dynamic config |

**Key rules:**
- Secrets (API keys, `SECRET_KEY_BASE`, DB credentials) go in `runtime.exs`
- Never commit secrets to `prod.exs`
- Use `System.get_env/1` only in `runtime.exs`

```elixir
# config/runtime.exs
if config_env() == :prod do
  config :my_app, MyApp.Repo,
    url: System.get_env("DATABASE_URL"),
    pool_size: String.to_integer(System.get_env("POOL_SIZE") || "10")
end
```

---

## Things to Avoid

| Don't | Do Instead |
|-------|------------|
| `String.to_atom(user_input)` | Use existing atoms or `String.to_existing_atom/1` |
| Nested modules in same file | One module per file |
| Raw `<script>` tags | Colocated hooks or external JS |
| `ilike` in queries | `fragment("lower(?) LIKE ?", ...)` for SQLite |
| Map access on structs | Dot notation: `struct.field` |
| `Process.sleep` in tests | `Process.monitor` or proper synchronization |

---

## Pre-Commit Checklist

Before committing:

1. `mix format` - Format all code
2. `mix compile --warnings-as-errors` - No compilation warnings
3. `mix credo --strict` - No Credo issues
4. `mix test` - All tests pass
5. `mix dialyzer` - No type errors

Or run: `mix precommit` (if configured)
