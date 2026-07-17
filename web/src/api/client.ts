import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { DeploymentService } from "../gen/wataridori/v1/wataridori_pb";

// Same origin: `wataridori serve` mounts the RPC routes and this UI on one
// port. In dev, vite proxies the service prefix to :8080.
const transport = createConnectTransport({ baseUrl: "/" });

export const client = createClient(DeploymentService, transport);
