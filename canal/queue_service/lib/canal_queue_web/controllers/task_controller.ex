defmodule CanalQueueWeb.TaskController do
  @moduledoc """
  Controller for managing tasks via the JSON API.
  """

  use CanalQueueWeb, :controller

  alias CanalQueue.Tasks
  alias CanalQueue.Tasks.Task

  action_fallback CanalQueueWeb.FallbackController

  @doc """
  Lists all tasks, optionally filtered by status and/or project.

  GET /api/tasks
  GET /api/tasks?status=pending
  GET /api/tasks?project=canal
  """
  @spec index(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def index(conn, params) do
    opts =
      []
      |> maybe_add_opt(:status, params["status"])
      |> maybe_add_opt(:project, params["project"])

    tasks = Tasks.list_tasks(opts)
    render(conn, :index, tasks: tasks)
  end

  @doc """
  Gets a single task by task_id.

  GET /api/tasks/:task_id
  """
  @spec show(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def show(conn, %{"task_id" => task_id}) do
    case Tasks.get_task(task_id) do
      nil -> {:error, :not_found}
      task -> render(conn, :show, task: task)
    end
  end

  @doc """
  Creates a new task.

  POST /api/tasks
  """
  @spec create(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def create(conn, params) do
    with {:ok, %Task{} = task} <- Tasks.create_task(params) do
      conn
      |> put_status(:created)
      |> render(:show, task: task)
    end
  end

  @doc """
  Claims the next pending task.

  POST /api/tasks/claim
  """
  @spec claim(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def claim(conn, params) do
    opts = maybe_add_opt([], :project, params["project"])

    case Tasks.claim_task(opts) do
      {:ok, task} ->
        render(conn, :show, task: task)

      {:error, :no_tasks_available} ->
        render(conn, :no_task)
    end
  end

  @doc """
  Marks a task as complete.

  POST /api/tasks/:task_id/done
  """
  @spec complete(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def complete(conn, %{"task_id" => task_id}) do
    with {:ok, task} <- Tasks.complete_task(task_id) do
      render(conn, :show, task: task)
    end
  end

  @doc """
  Marks a task as blocked.

  POST /api/tasks/:task_id/blocked
  """
  @spec block(Plug.Conn.t(), map()) :: Plug.Conn.t()
  def block(conn, %{"task_id" => task_id} = params) do
    reason = params["reason"] || ""

    with {:ok, task} <- Tasks.block_task(task_id, reason) do
      render(conn, :show, task: task)
    end
  end

  defp maybe_add_opt(opts, _key, nil), do: opts
  defp maybe_add_opt(opts, _key, ""), do: opts
  defp maybe_add_opt(opts, key, value), do: Keyword.put(opts, key, value)
end
