export const procedures = {
  status: "/wataridori.v1.DeploymentService/Status",
  history: "/wataridori.v1.DeploymentService/History",
  planPromote: "/wataridori.v1.DeploymentService/PlanPromote",
  apply: "/wataridori.v1.DeploymentService/Apply",
} as const;

interface ConnectErrorBody {
  message?: string;
  error?: string;
}

export async function rpc<TResponse>(path: string, body: unknown): Promise<TResponse> {
  const res = await fetch(path, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Connect-Protocol-Version": "1",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  const data = parseJSON(text);
  if (!res.ok) {
    const err = data as ConnectErrorBody;
    throw new Error(err.message || err.error || `${res.status} ${res.statusText}`);
  }
  return data as TResponse;
}

function parseJSON(text: string): unknown {
  if (!text) {
    return {};
  }
  return JSON.parse(text) as unknown;
}
