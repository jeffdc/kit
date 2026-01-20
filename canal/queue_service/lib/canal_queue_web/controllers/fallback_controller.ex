defmodule CanalQueueWeb.FallbackController do
  @moduledoc """
  Handles errors from controller actions.

  Translates common error tuples into appropriate HTTP responses.
  """

  use CanalQueueWeb, :controller

  @doc """
  Handles error responses from controller actions.

  Translates error tuples into appropriate HTTP responses:
  - `{:error, :not_found}` → 404 Not Found
  - `{:error, :invalid_status}` → 422 Unprocessable Entity
  - `{:error, %Ecto.Changeset{}}` → 422 Unprocessable Entity with validation errors
  """
  @spec call(Plug.Conn.t(), {:error, atom() | Ecto.Changeset.t()}) :: Plug.Conn.t()
  def call(conn, error)

  def call(conn, {:error, :not_found}) do
    conn
    |> put_status(:not_found)
    |> put_view(json: CanalQueueWeb.ErrorJSON)
    |> render(:"404")
  end

  def call(conn, {:error, :invalid_status}) do
    conn
    |> put_status(:unprocessable_entity)
    |> put_view(json: CanalQueueWeb.ErrorJSON)
    |> render(:invalid_status)
  end

  def call(conn, {:error, %Ecto.Changeset{} = changeset}) do
    conn
    |> put_status(:unprocessable_entity)
    |> put_view(json: CanalQueueWeb.ErrorJSON)
    |> render(:changeset_error, changeset: changeset)
  end
end
