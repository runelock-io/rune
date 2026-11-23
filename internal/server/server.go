package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"rune/internal/api"
	"rune/internal/audit"
	"rune/internal/auth"
	"rune/internal/config"
	"rune/internal/constants"
	"rune/internal/crypto"
	"rune/internal/middleware"
	"rune/internal/seal"
	"rune/internal/service"
	"rune/internal/storage"
	"rune/internal/ui"
	"time"

	"golang.org/x/term"
)

const port = constants.DefaultPort

const (
	ansiReset   = "\033[0m"
	ansiBold    = "\033[1m"
	ansiDim     = "\033[2m"
	ansiCyan    = "\033[96m"
	ansiBlue    = "\033[94m"
	ansiYellow  = "\033[33m"
	ansiGray    = "\033[90m"
	ansiMagenta = "\033[95m"
)

func printBanner(dbPath string) {
	fmt.Println()
	fmt.Printf("  %s%srune%s  %slocal-first secrets vault%s\n", ansiBold, ansiCyan, ansiReset, ansiDim, ansiReset)
	fmt.Println()
	fmt.Printf("  %s➜%s  %-6s %s0.0.0.0:%s%s %s(all interfaces)%s\n", ansiCyan, ansiReset, "api", ansiBlue, port, ansiReset, ansiDim, ansiReset)
	fmt.Printf("  %s➜%s  %-6s %shttp://localhost:%s/ui/%s\n",         ansiCyan, ansiReset, "ui",  ansiBold+ansiBlue, port, ansiReset)
	fmt.Printf("  %s➜%s  %-6s %s%s%s\n",                              ansiCyan, ansiReset, "db",  ansiGray, dbPath, ansiReset)
	fmt.Println()
	fmt.Printf("  %s⬡  vault sealed%s  —  run %srune unseal%s or open the web UI\n", ansiYellow, ansiReset, ansiBold, ansiReset)
	fmt.Println()
}

func Start() {
	log.SetFlags(0)

	dbPath, err := config.GetDBPath()
	if err != nil {
		log.Fatal(err)
	}

	store, err := storage.NewBoltStore(dbPath)
	if err != nil {
		log.Fatal(err)
	}

	salt, err := store.GetSalt()
	if err != nil {
		// Initialization
		salt, _ = crypto.GenerateSalt()
		store.SaveSalt(salt)

		fmt.Print("Set A Passphrase (Required For Sealing and Unsealing): ")
		passphraseBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			log.Fatal("Failed to read passphrase: ", err)
		}
		passphrase := string(passphraseBytes)

		key := crypto.DeriveKey(passphrase, salt)

		ciphertext, nonce, _ := crypto.Encrypt(key, []byte("rune-check"))

		store.Put(storage.VerifyKey, storage.SecretRecord{
			Ciphertext: ciphertext,
			Nonce:      nonce,
		})

		token, hash := auth.GenerateToken()
		record := auth.TokenRecord{
			ID:        "root",
			Hash:      hash,
			Name:      "root",
			Namespace: "*",
			CreatedAt: time.Now().Format(time.RFC3339),
			Revoked:   false,
		}
		err = store.SaveToken(record)
		if err != nil {
			return
		}

		sep := "  " + strings.Repeat("─", 72)
		fmt.Println()
		fmt.Println(sep)
		fmt.Printf("  %s%sroot token%s\n", ansiBold, ansiYellow, ansiReset)
		fmt.Println(sep)
		fmt.Println()
		fmt.Printf("  %s%s%s%s\n", ansiBold, ansiCyan, token, ansiReset)
		fmt.Println()
		fmt.Println(sep)
		fmt.Printf("  %sshown once  ·  not stored  ·  full vault access%s\n", ansiDim, ansiReset)
		fmt.Println(sep)
		fmt.Println()

		fmt.Println()
		fmt.Printf("  %s%s✦  You're one of the first people to run Rune — thank you!%s\n", ansiBold, ansiMagenta, ansiReset)
		fmt.Println()
		fmt.Printf("  %sRune is a passion project built for developers who take%s\n", ansiDim, ansiReset)
		fmt.Printf("  %ssecrets seriously. Your early trust means everything.%s\n", ansiDim, ansiReset)
		fmt.Println()
		fmt.Printf("  %sIf Rune is useful to you, a ★ on GitHub goes a long way%s\n", ansiDim, ansiReset)
		fmt.Printf("  %sand helps others discover the project.%s\n", ansiDim, ansiReset)
		fmt.Println()
		fmt.Printf("  %s→  https://github.com/runelock-io/rune%s\n", ansiCyan, ansiReset)
		fmt.Println()

	}

	store.CreateNamespace(constants.DefaultNamespace)

	sealer := seal.NewManger()
	secretService := service.NewSecretService(store, sealer)
	tokenService := service.NewTokenService(store, sealer)
	namespaceService := service.NewNamespaceService(store)
	k8sService := service.NewK8sMappingService(store)
	auditLogger := audit.NewLogger(store)
	handler := api.NewHandler(secretService, tokenService, namespaceService, store, sealer, auditLogger)
	k8sHandler := api.NewK8sHandler(k8sService, auditLogger)
	auditHandler := api.NewAuditHandler(store)

	// PUBLIC
	public := http.NewServeMux()
	public.HandleFunc("/unseal", handler.Unseal)
	public.HandleFunc("/status", handler.Status)
	public.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	// PROTECTED
	protected := http.NewServeMux()
	protected.HandleFunc("/secret/put", handler.PutSecret)
	protected.HandleFunc("/secret/get", handler.GetSecret)
	protected.HandleFunc("/secret/delete", handler.DeleteSecret)
	protected.HandleFunc("/secret/list", handler.ListSecrets)
	protected.HandleFunc("/secret/rotate", handler.RotateSecret)
	protected.HandleFunc("/seal", handler.Seal)
	protected.HandleFunc("/token/me", handler.GetCurrentToken)
	protected.HandleFunc("/token/create", handler.CreateToken)
	protected.HandleFunc("/token/create-system", handler.CreateSystemToken)
	protected.HandleFunc("/token/revoke", handler.RevokeToken)
	protected.HandleFunc("/token/list", handler.ListTokens)
	protected.HandleFunc("/namespace/create", handler.CreateNamespace)
	protected.HandleFunc("/namespace/list", handler.ListNamespaces)
	protected.HandleFunc("/namespace/delete", handler.DeleteNamespace)
	protected.HandleFunc("/secret/version", handler.GetSecretVersion)
	protected.HandleFunc("/secret/history", handler.ListSecretVersions)
	protected.HandleFunc("/secret/all", handler.GetAllSecrets)
	protected.HandleFunc("/secret/ttl", handler.SetSecretTTL)
	protected.HandleFunc("/secret/meta", handler.ListSecretMeta)
	protected.HandleFunc("/stats", handler.GetStats)
	protected.HandleFunc("/k8s/mappings", k8sHandler.ListMappings)
	protected.HandleFunc("/k8s/mapping/create", k8sHandler.CreateMapping)
	protected.HandleFunc("/k8s/mapping/delete", k8sHandler.DeleteMapping)
	protected.HandleFunc("/k8s/mapping/status", k8sHandler.UpdateStatus)
	protected.HandleFunc("/audit/events", auditHandler.ListEvents)

	// APPLY MIDDLEWARE
	secured := middleware.AuthMiddleware(store)(protected)

	finalMux := http.NewServeMux()
	finalMux.Handle("/unseal", public)
	finalMux.Handle("/status", public)
	finalMux.Handle("/health", public)
	finalMux.Handle("/ui/", http.StripPrefix("/ui", ui.Handler()))
	finalMux.Handle("/", secured)

	go startTTLScheduler(secretService, store, auditLogger, sealer)

	printBanner(dbPath)
	log.Fatal(http.ListenAndServe(":"+port, finalMux))
}

func startTTLScheduler(svc *service.SecretService, store storage.Store, al *audit.Logger, sealer *seal.Manager) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		if sealer.IsSealed() {
			continue
		}
		expired, err := store.ListExpiredSecrets()
		if err != nil || len(expired) == 0 {
			continue
		}
		for _, fullKey := range expired {
			parts := strings.SplitN(fullKey, "/", 2)
			if len(parts) != 2 {
				continue
			}
			ns, key := parts[0], parts[1]
			_, rotErr := svc.Rotate(ns, key)
			status := audit.StatusSuccess
			msg := ""
			if rotErr != nil {
				status = audit.StatusFailure
				msg = rotErr.Error()
			}
			al.Log(audit.Event{
				ID:        fmt.Sprintf("%020d", time.Now().UnixNano()),
				Timestamp: time.Now().Format(time.RFC3339),
				Type:      audit.TypeSecretRotate,
				Actor:     "rune-scheduler",
				ActorID:   "system",
				Resource:  fullKey,
				Status:    status,
				Message:   msg,
				IP:        "127.0.0.1",
			})
		}
	}
}
