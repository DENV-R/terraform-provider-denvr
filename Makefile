BINARY = terraform-provider-denvr
VERSION ?= 0.2.2

# Compile le provider.
build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY)

# Récupère/aligne les dépendances (requiert un accès réseau au proxy Go).
tidy:
	go mod tidy

# Vérifie la compilation et le formatage.
check:
	gofmt -l .
	go vet ./...

# Tests d'acceptance : créent de VRAIES ressources sur le compte de test cible par
# DENVR_API_URL / DENVR_TOKEN (jeton lecture-écriture). Sans TF_ACC, ils sont ignorés.
testacc:
	TF_ACC=1 go test ./... -v -timeout 30m

# Génère la documentation du Registry (docs/) depuis les schémas + examples/.
# Requiert tfplugindocs (go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest).
docs:
	tfplugindocs generate --provider-name denvr

# Installe le provider en local pour un essai (~/.terraform.d/plugins).
# Adaptez OS_ARCH à votre machine (ex. linux_amd64, darwin_arm64, windows_amd64).
OS_ARCH ?= linux_amd64
PLUGIN_DIR = ~/.terraform.d/plugins/registry.terraform.io/denv-r/denvr/$(VERSION)/$(OS_ARCH)

install: build
	mkdir -p $(PLUGIN_DIR)
	cp $(BINARY) $(PLUGIN_DIR)/

.PHONY: build tidy check testacc docs install
