# Fase 1: Build del Frontend
# Usiamo un'immagine con Node.js per poter usare 'npm' a 64bit per compilare la grafica
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend

# Copiamo prima solo i file delle dipendenze per velocizzare (sfruttando la cache di Docker)
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Ora copiamo tutto il resto del codice frontend e lanciamo la build (genera la cartella "dist")
COPY frontend/ ./
RUN npm run build


# Fase 2: Build del Backend (Go)
# Usiamo un'immagine con Golang per compilare il server web
FROM golang:alpine AS backend-builder
WORKDIR /app/backend

# Anche qui, prima i moduli per la cache
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copiamo il codice sorgente Go
COPY backend/ ./

# Compiliamo il programma. 
# CGO_ENABLED=0 disabilita le librerie in C (perché vogliamo un eseguibile standalone perfetto per alpine).
# GOOS=linux e GOARCH=arm64 assicurano che il codice sia compatibile con il Raspberry, anche se lo compiliamo dal tuo PC Windows.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o wol-server .


# Fase 3: L'Immagine Finale (quella che andrà effettivamente sul Raspberry)
# Usiamo un'immagine super-leggera (Alpine Linux) per risparmiare spazio sulla SD del Raspberry
FROM alpine:latest
WORKDIR /app

# Copiamo l'eseguibile Go dalla 'Fase 2'
COPY --from=backend-builder /app/backend/wol-server /app/wol-server

# Copiamo i file statici di React generati nella 'Fase 1'
# Ricordi? in main.go abbiamo detto a Go di cercare questi file in "./frontend/dist"
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

# Espone la porta che userà il nostro programma
EXPOSE 8080

# Specifichiamo qual è il comando finale per lanciare il server!
CMD ["/app/wol-server"]
