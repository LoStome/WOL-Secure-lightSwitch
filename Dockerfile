# Fase 1: Build del Frontend
# Usiamo un'immagine con Node.js per poter usare 'npm' a 64bit per compilare la grafica
FROM node:20.20.2-alpine3.23@sha256:fb4cd12c85ee03686f6af5362a0b0d56d50c58a04632e6c0fb8363f609372293 AS frontend-builder
WORKDIR /app/frontend

# Copiamo prima solo i file delle dipendenze per velocizzare (sfruttando la cache di Docker)
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Ora copiamo tutto il resto del codice frontend e lanciamo la build (genera la cartella "dist")
COPY frontend/ ./
RUN npm run build


# Fase 2: Build del Backend (Go)
# Usiamo un'immagine con Golang per compilare il server web
FROM golang:1.26.6-alpine3.24@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS backend-builder
WORKDIR /app/backend

# Diciamo a Docker che accettiamo argomenti sulla piattaforma target forniti da `buildx`
ARG TARGETOS
ARG TARGETARCH

# Anche qui, prima i moduli per la cache
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copiamo il codice sorgente Go
COPY backend/ ./

# Compiliamo il programma tenendo conto dell'architettura scelta al momento del build (amd64 o arm64)
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o wol-server .


# Fase 3: L'Immagine Finale
FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
WORKDIR /app

# Installiamo il pacchetto tzdata per gestire correttamente le timezone, ed eventuali certificati ca
# e creiamo l'utente senza privilegi che esegue il server.
RUN apk add --no-cache tzdata ca-certificates \
    && addgroup -S wol \
    && adduser -S -G wol -h /app -s /sbin/nologin wol \
    && mkdir -p /app/data \
    && chown -R wol:wol /app

# Copiamo l'eseguibile Go dalla 'Fase 2'
COPY --chown=wol:wol --from=backend-builder /app/backend/wol-server /app/wol-server

# Copiamo i file statici di React generati nella 'Fase 1'
# Ricordi? in main.go abbiamo detto a Go di cercare questi file in "./frontend/dist"
COPY --chown=wol:wol --from=frontend-builder /app/frontend/dist /app/frontend/dist

# Espone la porta che userà il nostro programma
EXPOSE 7500

# Il processo applicativo non deve avere privilegi root.
USER wol

# Specifichiamo qual è il comando finale per lanciare il server!
CMD ["/app/wol-server"]
