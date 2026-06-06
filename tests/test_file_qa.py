import json
import os
import tempfile
import unittest
import urllib.error
import urllib.request
import uuid


BASE_URL = os.getenv("AURORA_BASE_URL", "http://127.0.0.1:8080").rstrip("/")


def auth_header() -> str:
    header = os.getenv("AURORA_AUTH_HEADER", "").strip()
    if header:
        return header

    api_key = os.getenv("AURORA_API_KEY", "").strip()
    access_token = os.getenv("AURORA_ACCESS_TOKEN", "").strip()
    if api_key and access_token:
        return f"Bearer {api_key} {access_token}"
    if access_token:
        return f"Bearer {access_token}"
    if api_key:
        return f"Bearer {api_key}"
    return ""


def request_json(method: str, path: str, payload: dict, authorization: str) -> dict:
    body = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        f"{BASE_URL}{path}",
        data=body,
        method=method,
        headers={
            "Authorization": authorization,
            "Content-Type": "application/json",
            "Accept": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(request, timeout=180) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise AssertionError(f"{method} {path} failed: HTTP {exc.code}: {detail}") from exc


def upload_file(path: str, authorization: str, purpose: str = "assistants") -> dict:
    boundary = f"----aurora-test-{uuid.uuid4().hex}"
    filename = os.path.basename(path)
    with open(path, "rb") as handle:
        file_bytes = handle.read()

    fields = [
        (
            f"--{boundary}\r\n"
            'Content-Disposition: form-data; name="purpose"\r\n\r\n'
            f"{purpose}\r\n"
        ).encode("utf-8"),
        (
            f"--{boundary}\r\n"
            f'Content-Disposition: form-data; name="file"; filename="{filename}"\r\n'
            "Content-Type: text/plain\r\n\r\n"
        ).encode("utf-8"),
        file_bytes,
        f"\r\n--{boundary}--\r\n".encode("utf-8"),
    ]
    body = b"".join(fields)

    request = urllib.request.Request(
        f"{BASE_URL}/v1/files",
        data=body,
        method="POST",
        headers={
            "Authorization": authorization,
            "Content-Type": f"multipart/form-data; boundary={boundary}",
            "Accept": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(request, timeout=180) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        raise AssertionError(f"POST /v1/files failed: HTTP {exc.code}: {detail}") from exc


class FileQATest(unittest.TestCase):
    def test_upload_file_then_ask_question(self):
        authorization = auth_header()
        if not authorization:
            self.skipTest(
                "set AURORA_ACCESS_TOKEN, AURORA_API_KEY, or AURORA_AUTH_HEADER to run this integration test"
            )

        sentinel = f"aurora-file-qa-{uuid.uuid4().hex[:10]}"
        text = (
            "This is an Aurora file QA integration test.\n"
            f"The sentinel value is: {sentinel}\n"
            "When asked, answer with only the sentinel value.\n"
        )

        configured_path = os.getenv("AURORA_FILE_PATH", "").strip()
        if configured_path:
            file_path = configured_path
            cleanup = False
        else:
            temp = tempfile.NamedTemporaryFile("w", suffix=".txt", encoding="utf-8", delete=False)
            try:
                temp.write(text)
                file_path = temp.name
            finally:
                temp.close()
            cleanup = True

        try:
            uploaded = upload_file(file_path, authorization)
            file_id = uploaded.get("id") or uploaded.get("file_id")
            self.assertTrue(file_id, f"upload response did not include file id: {uploaded}")

            response = request_json(
                "POST",
                "/v1/chat/completions",
                {
                    "model": os.getenv("AURORA_MODEL", "auto"),
                    "stream": False,
                    "messages": [
                        {
                            "role": "user",
                            "content": [
                                {"type": "input_file", "file_id": file_id},
                                {
                                    "type": "text",
                                    "text": "What is the sentinel value in the uploaded file? Answer only the sentinel value.",
                                },
                            ],
                        }
                    ],
                },
                authorization,
            )
            answer = response["choices"][0]["message"]["content"]
            self.assertIn(sentinel, answer)
        finally:
            if cleanup:
                try:
                    os.unlink(file_path)
                except OSError:
                    pass


if __name__ == "__main__":
    unittest.main()
