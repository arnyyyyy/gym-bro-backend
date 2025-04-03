1. Перейдите в директорию проекта:
   ```bash
   cd gym-bro-backend
   ```

2. Запустите сервер с помощью Go:
   ```bash
   go run gymBroServer.go
   ```

3. В другом терминале выполните команду для проброса портов через Serveo:
   ```bash
   ssh -R gymbro.serveo.net:80:localhost:8080 serveo.net
   ```
