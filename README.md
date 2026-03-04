# web hosting - Go api

There is a web script hosting coded in Go language.

## database ( sqlite )

sessions:

- token: TEXT PRIMARY KEY
- user_id: INTEGER

users:

- id : INTEGER
- username : TEXT UNIQUE
- password : TEXT

scripts:

- id : TEXT PRIMARY KEY
- user_id : INTEGER
- name : TEXT
- description : TEXT
- language : TEXT
- docker_image : TEXT
- file_path : TEXT
- entrypoint : TEXT
- created_at : DATETIME

executions:

- id : TEXT PRIMARY KEY
- script_id : TEXT
- user_id : INTEGER
- status : TEXT (pending / running / success / failed)
- exit_code : INTEGER
- started_at : DATETIME
- finished_at : DATETIME

logs:

- id : TEXT PRIMARY KEY
- execution_id : TEXT
- stream : TEXT (stdout / stderr)
- content : TEXT
- created_at : DATETIME

## installation

```bash
set DOCKER_HOST=npipe:////./pipe/docker_engine

docker pull python:3.11-alpine
docker pull bash:5-alpine
docker pull node:20-alpine
```

## run

```bash
cd "C:\Users\Léo\Desktop\code\webhosting-goapi\dev_test"
go run ../cmd/api
```

## functions

- login:

    ```bash
    curl -X POST http://127.0.0.1:8000/login -H "Content-Type: application/json" -d "{\"username\": \"username\", \"password\": \"password\"}" -c cookies.txt
    ```

    `-c cookies.txt` save the cookie `session_token` automatically.

    response:

    ```json
    Logged in : kr0AEV1oaSXfPfBlYA7C9Qw5aDf_tgfTvhZN_5XsOkg=
    ```

    error (wrong credentials):

    ```json
    Invalid credentials
    ```

    ---

- logout:

    ```bash
    curl -X POST http://127.0.0.1:8000/logout -b cookies.txt
    ```

    response:

    ```json
    Logged out
    ```

    ---

- upload script (python):

    ```bash
    echo print("Hello from Docker!") > test.py

    curl -X POST http://127.0.0.1:8000/scripts/upload -b cookies.txt -F "name=Mon premier script" -F "description=Test Python" -F "language=python" -F "entrypoint=test.py" -F "file=@test.py"
    ```

    response:

    ```json
    {
        "id": "9352c1fb-4a39-48d0-8cf7-38756c6b7d3c",
        "entrypoint": "test.py",
        "files": ["test.py"],
        "message": "Script uploaded successfully"
    }
    ```

    ---

- upload script (bash):

    ```bash
    echo echo "Hello from Bash!" > test.sh

    curl -X POST http://127.0.0.1:8000/scripts/upload -b cookies.txt -F "name=Script bash" -F "language=bash" -F "entrypoint=test.sh" -F "file=@test.sh"
    ```

    response:

    ```json
    {
        "id": "bc9f70b8-19f6-4f9a-8759-8909b029423e",
        "entrypoint": "test.sh",
        "files": ["test.sh"],
        "message": "Script uploaded successfully"
    }
    ```

    ---

- upload script (node.js):

    ```bash
    echo console.log("Hello from Node!") > test.js

    curl -X POST http://127.0.0.1:8000/scripts/upload -b cookies.txt -F "name=Script JS" -F "language=nodejs" -F "entrypoint=test.js" -F "file=@test.js"
    ```

    response:

    ```json
    {
        "id": "10f9f5fb-598d-44ce-bab2-d0640bca2a7c",
        "entrypoint": "test.js",
        "files": ["test.js"],
        "message": "Script uploaded successfully"
    }
    ```

    ---

- upload zip archive (auto-extracted):

    ```bash
    curl -X POST http://127.0.0.1:8000/scripts/upload -b cookies.txt -F "name=Mon projet" -F "language=python" -F "entrypoint=src/main.py" -F "file=@project.zip"
    ```

    response:

    ```json
    {
        "id": "f3a1c2d4-8b9e-4f2a-a1b2-c3d4e5f6a7b8",
        "entrypoint": "src/main.py",
        "files": [
            "src/main.py",
            "src/utils.py",
            "requirements.txt"
        ],
        "message": "Script uploaded successfully"
    }
    ```

    ---

- upload multiple files:

    ```bash
    curl -X POST http://127.0.0.1:8000/scripts/upload -b cookies.txt -F "name=Multi" -F "language=python" -F "entrypoint=main.py" -F "file=@main.py" -F "file=@utils.py"
    ```

    response:

    ```json
    {
        "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
        "entrypoint": "main.py",
        "files": ["main.py", "utils.py"],
        "message": "Script uploaded successfully"
    }
    ```

    ---

- list all scripts:

    ```bash
    curl http://127.0.0.1:8000/scripts -b cookies.txt
    ```

    response:

    ```json
    [
        {
            "id": "9352c1fb-4a39-48d0-8cf7-38756c6b7d3c",
            "name": "Mon premier script",
            "description": "Test Python",
            "language": "python",
            "docker_image": "python:3.11-alpine",
            "entrypoint": "test.py",
            "created_at": "2026-03-03T22:50:36Z"
        },
        {
            "id": "bc9f70b8-19f6-4f9a-8759-8909b029423e",
            "name": "Script bash",
            "description": "",
            "language": "bash",
            "docker_image": "bash:5-alpine",
            "entrypoint": "test.sh",
            "created_at": "2026-03-03T22:51:50Z"
        }
    ]
    ```

    ---

- script details:

    ```bash
    curl http://127.0.0.1:8000/scripts/SCRIPT_ID -b cookies.txt
    ```

    response:

    ```json
    {
        "id": "f3a1c2d4-8b9e-4f2a-a1b2-c3d4e5f6a7b8",
        "name": "Mon projet",
        "description": "Test Python",
        "language": "python",
        "docker_image": "python:3.11-alpine",
        "entrypoint": "src/main.py",
        "created_at": "2026-03-03T22:50:36Z",
        "tree": [
            {"path": "src/main.py", "size": 1024},
            {"path": "src/utils.py", "size": 512},
            {"path": "requirements.txt", "size": 64}
        ]
    }
    ```

    error (not found or not yours):

    ```json
    Script not found
    ```

    ---

- run script:

    ```bash
    curl -X POST http://127.0.0.1:8000/scripts/SCRIPT_ID/run -b cookies.txt
    ```

    response:

    ```json
    {
        "execution_id": "d3fa3368-de0a-4f2e-837d-74baa42a6798",
        "status": "running"
    }
    ```

    ---

- get script execution state:

    ```bash
    curl http://127.0.0.1:8000/executions/EXECUTION_ID -b cookies.txt
    ```

    response (running):

    ```json
    {
        "id": "d3fa3368-de0a-4f2e-837d-74baa42a6798",
        "script_id": "9352c1fb-4a39-48d0-8cf7-38756c6b7d3c",
        "status": "running",
        "exit_code": null,
        "started_at": "2026-03-03T22:52:18Z",
        "finished_at": null
    }
    ```

    response (success):

    ```json
    {
        "id": "d3fa3368-de0a-4f2e-837d-74baa42a6798",
        "script_id": "9352c1fb-4a39-48d0-8cf7-38756c6b7d3c",
        "status": "success",
        "exit_code": 0,
        "started_at": "2026-03-03T22:52:18Z",
        "finished_at": "2026-03-03T22:52:21Z"
    }
    ```

    response (failed):

    ```json
    {
        "id": "d3fa3368-de0a-4f2e-837d-74baa42a6798",
        "script_id": "9352c1fb-4a39-48d0-8cf7-38756c6b7d3c",
        "status": "failed",
        "exit_code": 1,
        "started_at": "2026-03-03T22:52:18Z",
        "finished_at": "2026-03-03T22:52:19Z"
    }
    ```

    ---

- get script executions logs (json):

    ```bash
    curl http://127.0.0.1:8000/executions/EXECUTION_ID/logs -b cookies.txt
    ```

    response:

    ```json
    [
        {
            "stream": "stdout",
            "content": "Hello from Docker!",
            "created_at": "2026-03-03T22:52:21Z"
        }
    ]
    ```

    response (with stderr):

    ```json
    [
        {"stream": "stdout", "content": "Step 1/3", "created_at": "2026-03-03T22:52:19Z"},
        {"stream": "stdout", "content": "Step 2/3", "created_at": "2026-03-03T22:52:20Z"},
        {"stream": "stderr", "content": "FileNotFoundError: config.json not found", "created_at": "2026-03-03T22:52:21Z"}
    ]
    ```

    ---

- get script executions logs (raw text):

    ```bash
    curl "http://127.0.0.1:8000/executions/EXECUTION_ID/logs?format=text" -b cookies.txt
    ```

    response:

    ```json
    [stdout] Hello from Docker!
    ```

    response (with stderr):

    ```json
    [stdout] Step 1/3
    [stdout] Step 2/3
    [stderr] FileNotFoundError: config.json not found
    ```

    ---

- connect to execution logs stream (SSE):

    ```bash
    curl -N http://127.0.0.1:8000/executions/EXECUTION_ID/stream -b cookies.txt
    ```

    response (live, line by line):

    ```json
    event: log
    data: {"stream":"stdout","content":"Step 1/10"}

    event: log
    data: {"stream":"stdout","content":"Step 2/10"}

    event: log
    data: {"stream":"stdout","content":"Step 3/10"}

    event: done
    data: {"status":"success","exit_code":0}
    ```

    response (script already finished — replays from DB then closes):

    ```json
    event: log
    data: {"stream":"stdout","content":"Hello from Docker!"}

    event: done
    data: {"status":"success","exit_code":0}
    ```

    response (script failed):

    ```json
    event: log
    data: {"stream":"stderr","content":"Traceback (most recent call last):"}

    event: log
    data: {"stream":"stderr","content":"  NameError: name 'x' is not defined"}

    event: done
    data: {"status":"failed","exit_code":1}
    ```

    ---

- delete a script:

    ```bash
    curl -X DELETE http://127.0.0.1:8000/scripts/SCRIPT_ID -b cookies.txt
    ```

    response: `HTTP 204 No Content` (empty body)

    error (not found or not yours):

    ```json
    Script not found
    ```
