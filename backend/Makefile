.PHONY: run build test fmt vet tidy clean

# Exécute l'API en développement local
run:
	go run ./cmd/api/main.go

# Compile le binaire de production
build:
	go build -o bin/api ./cmd/api/main.go

# Lance tous les tests unitaires avec couverture
test:
	go test -v -race ./...

# Formate le code Go selon les standards
fmt:
	go fmt ./...

# Analyse statique du code Go
vet:
	go vet ./...

# Télécharge et nettoie les dépendances du go.mod
tidy:
	go mod tidy

# Nettoie les fichiers compilés
clean:
	rm -rf bin/
