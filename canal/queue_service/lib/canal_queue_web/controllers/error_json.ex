defmodule CanalQueueWeb.ErrorJSON do
  @moduledoc """
  This module is invoked by your endpoint in case of errors on JSON requests.

  See config/config.exs.
  """

  @doc """
  Renders error responses.

  Handles specific error templates (invalid_status, changeset_error) and
  falls back to Phoenix's default status message for other templates.
  """
  def render(template, assigns)

  def render("invalid_status.json", _assigns) do
    %{errors: %{detail: "Task is not in a valid state for this operation"}}
  end

  def render("changeset_error.json", %{changeset: changeset}) do
    %{errors: Ecto.Changeset.traverse_errors(changeset, &translate_error/1)}
  end

  def render(template, _assigns) do
    %{errors: %{detail: Phoenix.Controller.status_message_from_template(template)}}
  end

  defp translate_error({msg, opts}) do
    Enum.reduce(opts, msg, fn {key, value}, acc ->
      String.replace(acc, "%{#{key}}", fn _ -> to_string(value) end)
    end)
  end
end
