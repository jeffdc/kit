defmodule CanalQueueWeb.TaskJSON do
  @moduledoc """
  JSON rendering for tasks.
  """

  alias CanalQueue.Tasks.Task

  @doc """
  Renders a list of tasks.
  """
  @spec index(%{tasks: [Task.t()]}) :: %{tasks: [map()]}
  def index(%{tasks: tasks}) do
    %{tasks: for(task <- tasks, do: data(task))}
  end

  @doc """
  Renders a single task.
  """
  @spec show(%{task: Task.t()}) :: map()
  def show(%{task: task}) do
    data(task)
  end

  @doc """
  Renders a response when no task is available.
  """
  @spec no_task(map()) :: %{task: nil}
  def no_task(_assigns) do
    %{task: nil}
  end

  defp data(%Task{} = task) do
    %{
      task_id: task.task_id,
      project: task.project,
      status: task.status,
      spec_path: task.spec_path,
      origin: task.origin,
      blocked_reason: task.blocked_reason,
      claimed_at: task.claimed_at,
      completed_at: task.completed_at,
      inserted_at: task.inserted_at,
      updated_at: task.updated_at
    }
  end
end
