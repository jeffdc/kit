defmodule CanalQueue.Tasks do
  @moduledoc """
  The Tasks context.

  Provides functions for managing tasks in the Canal queue, including
  creating, claiming, completing, and blocking tasks.
  """

  import Ecto.Query
  alias CanalQueue.Repo
  alias CanalQueue.Tasks.Task

  @doc """
  Returns the list of tasks, optionally filtered by status and/or project.

  ## Options

    * `:status` - Filter by task status
    * `:project` - Filter by project

  ## Examples

      iex> list_tasks()
      [%Task{}, ...]

      iex> list_tasks(status: "pending", project: "canal")
      [%Task{}, ...]

  """
  @spec list_tasks(keyword()) :: [Task.t()]
  def list_tasks(opts \\ []) do
    Task
    |> filter_by_status(opts[:status])
    |> filter_by_project(opts[:project])
    |> order_by([t], asc: t.inserted_at)
    |> Repo.all()
  end

  defp filter_by_status(query, nil), do: query
  defp filter_by_status(query, status), do: where(query, [t], t.status == ^status)

  defp filter_by_project(query, nil), do: query
  defp filter_by_project(query, project), do: where(query, [t], t.project == ^project)

  @doc """
  Gets a single task by task_id.

  Returns `nil` if the task does not exist.

  ## Examples

      iex> get_task("canal-0001")
      %Task{}

      iex> get_task("nonexistent")
      nil

  """
  @spec get_task(String.t()) :: Task.t() | nil
  def get_task(task_id) do
    Repo.get_by(Task, task_id: task_id)
  end

  @doc """
  Gets a single task by task_id, raising if not found.

  ## Examples

      iex> get_task!("canal-0001")
      %Task{}

      iex> get_task!("nonexistent")
      ** (Ecto.NoResultsError)

  """
  @spec get_task!(String.t()) :: Task.t()
  def get_task!(task_id) do
    Repo.get_by!(Task, task_id: task_id)
  end

  @doc """
  Creates a new task.

  ## Examples

      iex> create_task(%{task_id: "canal-0001", project: "canal", spec_path: "plan/specs/canal-0001.md", origin: "planner"})
      {:ok, %Task{}}

      iex> create_task(%{})
      {:error, %Ecto.Changeset{}}

  """
  @spec create_task(map()) :: {:ok, Task.t()} | {:error, Ecto.Changeset.t()}
  def create_task(attrs) do
    %Task{}
    |> Task.changeset(attrs)
    |> Repo.insert()
  end

  @doc """
  Claims the next pending task (FIFO).

  Optionally filters by project. Returns `{:ok, task}` if a task was claimed,
  or `{:error, :no_tasks_available}` if there are no pending tasks.

  Uses a database lock to ensure exactly-once claiming.

  ## Examples

      iex> claim_task()
      {:ok, %Task{status: "claimed"}}

      iex> claim_task(project: "canal")
      {:ok, %Task{status: "claimed"}}

      iex> claim_task()
      {:error, :no_tasks_available}

  """
  @spec claim_task(keyword()) :: {:ok, Task.t()} | {:error, :no_tasks_available}
  def claim_task(opts \\ []) do
    # SQLite doesn't support row-level locking, so we use an optimistic approach:
    # 1. Find the first pending task
    # 2. Attempt to update it with a WHERE clause that checks it's still pending
    # 3. If the update affected 0 rows, someone else claimed it - return no tasks
    query =
      Task
      |> where([t], t.status == "pending")
      |> filter_by_project(opts[:project])
      |> order_by([t], asc: t.inserted_at)
      |> limit(1)

    case Repo.one(query) do
      nil ->
        {:error, :no_tasks_available}

      task ->
        now = DateTime.utc_now() |> DateTime.truncate(:second)

        # Optimistic update - only succeeds if status is still pending
        {count, _} =
          Task
          |> where([t], t.id == ^task.id and t.status == "pending")
          |> Repo.update_all(set: [status: "claimed", claimed_at: now, updated_at: now])

        if count == 1 do
          {:ok, Repo.get!(Task, task.id)}
        else
          # Someone else claimed it, try again or return no tasks
          {:error, :no_tasks_available}
        end
    end
  end

  @doc """
  Marks a task as complete.

  Returns `{:ok, task}` if successful, `{:error, :not_found}` if the task
  doesn't exist, or `{:error, :invalid_status}` if the task is not claimed.

  ## Examples

      iex> complete_task("canal-0001")
      {:ok, %Task{status: "done"}}

      iex> complete_task("nonexistent")
      {:error, :not_found}

  """
  @spec complete_task(String.t()) :: {:ok, Task.t()} | {:error, :not_found | :invalid_status}
  def complete_task(task_id) do
    case get_task(task_id) do
      nil ->
        {:error, :not_found}

      %Task{status: "claimed"} = task ->
        task
        |> Task.complete_changeset()
        |> Repo.update()

      %Task{} ->
        {:error, :invalid_status}
    end
  end

  @doc """
  Marks a task as blocked with a reason.

  Returns `{:ok, task}` if successful, `{:error, :not_found}` if the task
  doesn't exist, or `{:error, :invalid_status}` if the task is not claimed.

  ## Examples

      iex> block_task("canal-0001", "Need clarification on requirements")
      {:ok, %Task{status: "blocked"}}

      iex> block_task("nonexistent", "reason")
      {:error, :not_found}

  """
  @spec block_task(String.t(), String.t()) ::
          {:ok, Task.t()} | {:error, :not_found | :invalid_status}
  def block_task(task_id, reason) do
    case get_task(task_id) do
      nil ->
        {:error, :not_found}

      %Task{status: "claimed"} = task ->
        task
        |> Task.block_changeset(reason)
        |> Repo.update()

      %Task{} ->
        {:error, :invalid_status}
    end
  end
end
