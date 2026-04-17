import http from "k6/http";
import { check, sleep } from "k6";
import { SharedArray } from "k6/data";

// Load and parse the JSONL file once using SharedArray to save memory across VUs
const testCases = new SharedArray("api test cases", function () {
  const fileContent = open("../hack/problem_a_generate.jsonl");
  return fileContent
    .split("\n")
    .filter((line) => line.trim().length > 0)
    .map((line) => JSON.parse(line));
});

export const options = {
  vus: 10,
  duration: "30s",
  setResponseCallback: http.expectedStatuses(200, 400),
};

export default function () {
  // Pick a random test case for this iteration
  const randomIdx = Math.floor(Math.random() * testCases.length);
  const tc = testCases[randomIdx];

  const url = "http://localhost:8080/score";
  const payload = JSON.stringify(tc.input);
  const params = {
    headers: { "Content-Type": "application/json" },
  };

  // Send the POST request
  const res = http.post(url, payload, params);

  // Validate the status code against the expected status_code
  const isStatusCorrect = check(res, {
    [`status was ${tc.status_code}`]: (r) => r.status === tc.status_code,
  });

  // Optional: check response body if it was a successful 200 response
  if (tc.status_code === 200 && res.status === 200) {
    let body;
    try {
      body = res.json();
    } catch (e) {
      // JSON parse failed
    }

    check(body, {
      "has correct case_id": (b) => b && b.case_id === tc.output.case_id,
      "has correct tier": (b) => b && b.tier === tc.output.tier,
      // Optional: you can add a score check here as well, being mindful of floating point comparison if necessary
    });
  }

  // Small sleep to avoid completely overwhelming the local system if the API responds extremely fast
  sleep(0.01);
}
