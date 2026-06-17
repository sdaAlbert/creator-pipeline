import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: 20,
  duration: "30s",
};

export default function () {
  const payload = JSON.stringify({
    user_id: "k6-user",
    prompt: `commercial video ${__VU}-${__ITER}`,
    idempotency_key: `k6-${__VU}-${__ITER}`,
  });

  const res = http.post("http://localhost:8080/api/v1/creations", payload, {
    headers: { "Content-Type": "application/json" },
  });

  check(res, {
    "accepted": (r) => r.status === 202,
  });

  sleep(1);
}
