
# web hosting - Go api

There is a web script hosting coded in Go language.

## api functions

- `/login (username string, password string)`: return a session hash64 that never expire until you logout
- `/logout (hash64 string)`: logout your session and delete the hash
- `/server_list (hash64 string)`: return a server list and their states ( 0: off, 1: on, 2: blocking error )
- `/server (name string, hash64 string)`: return a server infos ( id, space, usedspace, stdin, stdout, stderr)
- `profile`

## api database ( sqlite )

sessions:
    - token: TEXT PRIMARY KEY
    - user_id: INTEGER

users:
    - id : INTEGER
    - username : TEXT UNIQUE
    - password : TEXT

api tests:

```bash
cd "C:\Users\Léo\Desktop\code\webhosting-goapi\dev_test"
go run ../cmd/api
```

- login:

    ```bash
    curl -X POST http://127.0.0.1:8000/login -H "Content-Type: application/json" -d "{\"username\": \"root\", \"password\": \"root\"}" -c cookies.txt
    ```

    ``-c cookies.txt`` save the cookie ``session_token`` automatically.

- logout:

    ```bash
    curl -X POST http://127.0.0.1:8000/logout -b cookies.txt
    ```

- upload script (python):

    ```bash
    echo 'print("Hello from Docker!")' > test.py

    curl -X POST http://127.0.0.1:8000/scripts/upload -b cookies.txt -F "name=Mon premier script" -F "description=Test Python" -F "language=python" -F "file=@test.py"
    ```

- upload script (bash):

    ```bash
    echo 'echo "Hello from Bash!"' > test.sh

    curl -X POST http://127.0.0.1:8000/scripts/upload -b cookies.txt -F "name=Script bash" -F "language=bash" -F "file=@test.sh"
    ```

- upload script (node.js):

    ```bash
    echo 'console.log("Hello from Node!")' > test.js

    curl -X POST http://127.0.0.1:8000/scripts/upload -b cookies.txt -F "name=Script JS" -F "language=nodejs" -F "file=@test.js"
    ```

- list all scripts:

    ```bash
    curl http://127.0.0.1:8000/scripts -b cookies.txt
    ```

- script details:

    ```bash
    curl http://127.0.0.1:8000/scripts/TON_SCRIPT_ID -b cookies.txt
    ```

- run script:

    ```bash
    curl -X POST http://127.0.0.1:8000/scripts/TON_SCRIPT_ID/run -b cookies.txt
    ```

    return ``execution_id``

- get script execution state:

    ```bash
    curl http://127.0.0.1:8000/executions/TON_EXECUTION_ID -b cookies.txt
    ```

- get script executions logs (json):

    ```bash
    curl http://127.0.0.1:8000/executions/TON_EXECUTION_ID/logs -b cookies.txt
    ```

- get script executions logs (raw text):

    ```bash
    curl "http://127.0.0.1:8000/executions/TON_EXECUTION_ID/logs?format=text" -b cookies.txt
    ```

- delete a script:

    ```bash
    curl -X DELETE http://127.0.0.1:8000/scripts/TON_SCRIPT_ID -b cookies.txt
    ```
