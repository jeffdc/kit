defmodule CanalQueue.Tasks.Task do
  @moduledoc """
  A task in the Canal queue.

  Tasks represent units of work that workers can claim and execute.
  """

  use Ecto.Schema
  import Ecto.Changeset

  @type t :: %__MODULE__{
          id: integer() | nil,
          task_id: String.t(),
          project: String.t(),
          status: String.t(),
          spec_path: String.t(),
          origin: String.t(),
          blocked_reason: String.t() | nil,
          claimed_at: DateTime.t() | nil,
          completed_at: DateTime.t() | nil,
          inserted_at: DateTime.t() | nil,
          updated_at: DateTime.t() | nil
        }

  @statuses ~w(pending claimed done blocked)

  schema "tasks" do
    field :task_id, :string
    field :project, :string
    field :status, :string, default: "pending"
    field :spec_path, :string
    field :origin, :string
    field :blocked_reason, :string
    field :claimed_at, :utc_datetime
    field :completed_at, :utc_datetime

    timestamps(type: :utc_datetime)
  end

  @doc """
  Creates a changeset for a new task.
  """
  @spec changeset(t(), map()) :: Ecto.Changeset.t()
  def changeset(task, attrs) do
    task
    |> cast(attrs, [:task_id, :project, :spec_path, :origin])
    |> validate_required([:task_id, :project, :spec_path, :origin])
    |> unique_constraint(:task_id)
  end

  @doc """
  Creates a changeset for claiming a task.
  """
  @spec claim_changeset(t()) :: Ecto.Changeset.t()
  def claim_changeset(task) do
    task
    |> change(status: "claimed", claimed_at: DateTime.utc_now() |> DateTime.truncate(:second))
  end

  @doc """
  Creates a changeset for completing a task.
  """
  @spec complete_changeset(t()) :: Ecto.Changeset.t()
  def complete_changeset(task) do
    task
    |> change(status: "done", completed_at: DateTime.utc_now() |> DateTime.truncate(:second))
  end

  @doc """
  Creates a changeset for blocking a task.
  """
  @spec block_changeset(t(), String.t()) :: Ecto.Changeset.t()
  def block_changeset(task, reason) do
    task
    |> change(status: "blocked", blocked_reason: reason)
  end

  @doc """
  Returns the list of valid statuses.
  """
  @spec statuses() :: [String.t()]
  def statuses, do: @statuses
end
