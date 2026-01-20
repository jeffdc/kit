defmodule CanalQueue.Repo.Migrations.CreateTasks do
  use Ecto.Migration

  def change do
    create table(:tasks) do
      add :task_id, :string, null: false
      add :project, :string, null: false
      add :status, :string, null: false, default: "pending"
      add :spec_path, :string, null: false
      add :origin, :string, null: false
      add :blocked_reason, :text
      add :claimed_at, :utc_datetime
      add :completed_at, :utc_datetime

      timestamps(type: :utc_datetime)
    end

    create unique_index(:tasks, [:task_id])
    create index(:tasks, [:status])
    create index(:tasks, [:project])
  end
end
