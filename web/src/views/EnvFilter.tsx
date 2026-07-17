import { useQuery } from "@tanstack/react-query";
import { useId } from "react";

import { client } from "../api/client";

interface Props {
  value: string;
  onChange: (env: string) => void;
}

/**
 * EnvFilter is a select backed by ListEnvironments. Environment names are
 * configuration, not free text, so the UI never asks the user to type one.
 */
export function EnvFilter({ value, onChange }: Props) {
  const id = useId();
  const query = useQuery({
    queryKey: ["environments"],
    queryFn: () => client.listEnvironments({}),
  });

  return (
    <>
      <label htmlFor={id}>Environment</label>
      <select id={id} value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">All</option>
        {(query.data?.environments ?? []).map((env) => (
          <option key={env.name} value={env.name}>
            {env.name}
          </option>
        ))}
      </select>
    </>
  );
}
