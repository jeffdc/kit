defmodule CanalQueue.Repo do
  use Ecto.Repo,
    otp_app: :canal_queue,
    adapter: Ecto.Adapters.SQLite3
end
