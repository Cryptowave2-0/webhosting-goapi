
# web hosting - Go api

There is a web script hosting coded in Go language.

## api functions

- `/login (username string, password string)`: return a session hash64 that never expire until you logout
- `/logout (hash64 string)`: logout your session and delete the hash
- `/server_list (hash64 string)`: return a server list and their states ( 0: off, 1: on, 2: blocking error )
- `/server (name string, hash64 string)`: return a server infos ( id, space, usedspace, stdin, stdout, stderr)
