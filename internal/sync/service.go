package sync

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"apps.z7.ai/usm/internal/usm"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"

	libp2p "github.com/libp2p/go-libp2p"
)

// ServiceConfig holds the configuration for the sync service
type ServiceConfig struct {
	PeerKeyPath      string
	TrustedPeersPath string
	StorageRoot      string      // usm storage root for vault paths
	SyncMode         string      // "disabled", "relaxed", "strict"
	Storage          usm.Storage // for catalogue and vault file access
}

// PairRequestCallback is called when a remote peer initiates pairing.
// It receives the remote peer ID and label, and must return the code
// entered by the local user and whether they accepted the request.
// This function blocks until the user responds.
type PairRequestCallback func(peerID, peerLabel string) (code string, accepted bool)

// SyncNotifyCallback is called on the responder when an incoming sync is about
// to begin. It blocks until the UI is ready (e.g. a lock dialog is displayed),
// then returns a doneFn that the service calls when the sync finishes. The doneFn
// receives nil on success or an error on failure, allowing the UI to dismiss the
// lock and reload state.
type SyncNotifyCallback func(peerID string) (doneFn func(err error))

// PairResult contains the outcome of a pairing attempt
type PairResult struct {
	PeerID  string
	Label   string
	Success bool
	Err     error
}

// pairTimeout is the deadline for a complete pairing exchange
const pairTimeout = 60 * time.Second

// Intervals for the background maintenance loop
const (
	peerSweepInterval   = 30 * time.Second // how often to remove stale peers
	mdnsRestartInterval = 2 * time.Minute  // how often to restart mDNS for network recovery
	peerMaxAge          = 3 * time.Minute  // remove peers not seen within this window
)

// Service manages the sync subsystem lifecycle
type Service struct {
	config         ServiceConfig
	mu             sync.Mutex
	host           host.Host
	mdnsSvc        mdns.Service
	peerList       *PeerList
	trustStore     *TrustStore
	running        bool
	done           chan struct{} // signals background loop to stop
	onPairRequest  PairRequestCallback
	onIncomingSync SyncNotifyCallback
}

// NewService creates a new sync service
func NewService(config ServiceConfig) (*Service, error) {
	ts, err := NewTrustStore(config.TrustedPeersPath)
	if err != nil {
		return nil, fmt.Errorf("could not load trust store: %w", err)
	}

	return &Service{
		config:     config,
		peerList:   NewPeerList(),
		trustStore: ts,
	}, nil
}

// Start initializes the libp2p host and mDNS responder.
// In disabled mode this is a no-op. Lifecycle is managed via Stop(),
// not through context cancellation — ctx is reserved for future use.
func (s *Service) Start(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if s.config.SyncMode == usm.SyncModeDisabled {
		return nil
	}

	// Load or create Ed25519 identity
	priv, err := LoadOrCreateIdentity(s.config.PeerKeyPath)
	if err != nil {
		return fmt.Errorf("could not load peer identity: %w", err)
	}

	// Create libp2p host — local TCP only, no relay, no DHT
	h, err := libp2p.New(
		libp2p.Identity(priv),
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
	)
	if err != nil {
		return fmt.Errorf("could not create libp2p host: %w", err)
	}
	s.host = h

	// Register stream handlers
	h.SetStreamHandler(PairProtocol, s.handlePairStream)
	h.SetStreamHandler(SyncProtocol, s.handleSyncStream)

	// Start mDNS responder
	notifee := &discoveryNotifee{
		selfID:     h.ID(),
		host:       h,
		peerList:   s.peerList,
		trustStore: s.trustStore,
	}
	svc := mdns.NewMdnsService(h, MDNSService, notifee)
	if err := svc.Start(); err != nil {
		_ = h.Close()
		return fmt.Errorf("could not start mDNS: %w", err)
	}
	s.mdnsSvc = svc

	// ATTN: background loop handles mDNS restart (recovers from sleep/network
	// changes) and peer list sweeping (removes peers not re-discovered in time).
	s.done = make(chan struct{})
	go s.backgroundLoop()

	s.running = true
	return nil
}

// Stop tears down the libp2p host and mDNS service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	// Signal background loop to exit
	if s.done != nil {
		close(s.done)
		s.done = nil
	}

	if s.mdnsSvc != nil {
		_ = s.mdnsSvc.Close()
		s.mdnsSvc = nil
	}

	if s.host != nil {
		_ = s.host.Close()
		s.host = nil
	}

	s.running = false
	return nil
}

// backgroundLoop periodically sweeps stale peers and restarts mDNS to recover
// from network changes (sleep, WiFi reconnect, IP change). It runs until the
// done channel is closed by Stop().
func (s *Service) backgroundLoop() {
	sweepTicker := time.NewTicker(peerSweepInterval)
	mdnsTicker := time.NewTicker(mdnsRestartInterval)
	defer sweepTicker.Stop()
	defer mdnsTicker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-sweepTicker.C:
			s.peerList.Sweep(peerMaxAge)
		case <-mdnsTicker.C:
			// ATTN: restart mDNS to rebind to current network interfaces.
			// This is the only reliable way to recover from sleep/WiFi changes
			// without OS-specific network monitoring.
			s.restartMDNS()
		}
	}
}

// restartMDNS closes the current mDNS service and creates a new one bound to
// the current network interfaces. This recovers discovery after sleep, WiFi
// reconnect, or IP address changes.
func (s *Service) restartMDNS() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.host == nil {
		return
	}

	if s.mdnsSvc != nil {
		_ = s.mdnsSvc.Close()
	}

	notifee := &discoveryNotifee{
		selfID:     s.host.ID(),
		host:       s.host,
		peerList:   s.peerList,
		trustStore: s.trustStore,
	}
	svc := mdns.NewMdnsService(s.host, MDNSService, notifee)
	if err := svc.Start(); err != nil {
		log.Println("mDNS restart failed:", err)
		return
	}
	s.mdnsSvc = svc
	log.Println("mDNS restarted for network recovery")
}

// Peers returns the current list of discovered peers
func (s *Service) Peers() []PeerInfo {
	return s.peerList.All()
}

// OnPeerChange registers a callback for peer list updates
func (s *Service) OnPeerChange(fn func([]PeerInfo)) {
	s.peerList.OnChange(fn)
}

// TrustStore returns the trust store for external use (pairing UI, etc.)
func (s *Service) TrustStore() *TrustStore {
	return s.trustStore
}

// SetTrusted updates a peer's trusted status in the peer list
func (s *Service) SetTrusted(peerID string, trusted bool, label string) {
	s.peerList.SetTrusted(peerID, trusted, label)
}

// RemovePeer removes a stale peer from the discovered list
func (s *Service) RemovePeer(peerID string) {
	s.peerList.Remove(peerID)
}

// IsRunning returns whether the service is active
func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// HostID returns the libp2p peer ID string, or empty if not running
func (s *Service) HostID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.host == nil {
		return ""
	}
	return s.host.ID().String()
}

// SetPairRequestCallback registers a handler for incoming pairing requests
func (s *Service) SetPairRequestCallback(fn PairRequestCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onPairRequest = fn
}

// SetSyncNotifyCallback registers a handler called when an incoming sync begins
// on this device (responder side). The handler blocks until the UI is locked,
// then returns a function the service calls when the sync finishes.
func (s *Service) SetSyncNotifyCallback(fn SyncNotifyCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onIncomingSync = fn
}

// InitiatePairing starts a pairing handshake with a remote peer.
// Returns the pairing code to display and a channel that delivers the result.
func (s *Service) InitiatePairing(ctx context.Context, peerID string) (string, <-chan PairResult, error) {
	s.mu.Lock()
	h := s.host
	ts := s.trustStore
	pl := s.peerList
	s.mu.Unlock()

	if h == nil {
		return "", nil, fmt.Errorf("sync service is not running")
	}

	code, _, err := GeneratePairingCode()
	if err != nil {
		return "", nil, fmt.Errorf("could not generate pairing code: %w", err)
	}

	pid, err := peer.Decode(peerID)
	if err != nil {
		return "", nil, fmt.Errorf("invalid peer ID: %w", err)
	}

	// ATTN: generate a 32-byte nonce for HMAC challenge — the code never
	// travels in plaintext, only HMAC(code, nonce) is exchanged.
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return "", nil, fmt.Errorf("could not generate nonce: %w", err)
	}

	resultCh := make(chan PairResult, 1)

	go func() {
		defer close(resultCh)

		stream, err := h.NewStream(ctx, pid, PairProtocol)
		if err != nil {
			resultCh <- PairResult{PeerID: peerID, Err: fmt.Errorf("could not connect: %w", err)}
			return
		}
		defer stream.Close()

		_ = stream.SetDeadline(time.Now().Add(pairTimeout))

		enc := json.NewEncoder(stream)
		dec := json.NewDecoder(stream)

		// Send pair request with our label and HMAC nonce
		reqPayload, _ := json.Marshal(PairRequestPayload{
			Label: localLabel(h),
			Nonce: base64.StdEncoding.EncodeToString(nonce),
		})
		if err := enc.Encode(Message{Action: ActionPairRequest, Payload: reqPayload}); err != nil {
			resultCh <- PairResult{PeerID: peerID, Err: fmt.Errorf("could not send request: %w", err)}
			return
		}

		// Wait for response with HMAC proof
		var resp Message
		if err := dec.Decode(&resp); err != nil {
			resultCh <- PairResult{PeerID: peerID, Err: fmt.Errorf("peer did not respond: %w", err)}
			return
		}

		if resp.Action != ActionPairResponse {
			resultCh <- PairResult{PeerID: peerID, Err: fmt.Errorf("unexpected response: %s", resp.Action)}
			return
		}

		var respPayload PairResponsePayload
		if err := json.Unmarshal(resp.Payload, &respPayload); err != nil {
			resultCh <- PairResult{PeerID: peerID, Err: fmt.Errorf("invalid response: %w", err)}
			return
		}

		// ATTN: verify HMAC — the responder computed HMAC(code_bytes, nonce)
		// and sent the MAC. We recompute and compare in constant time.
		receivedMAC, err := base64.StdEncoding.DecodeString(respPayload.MAC)
		if err != nil {
			resultCh <- PairResult{PeerID: peerID, Err: fmt.Errorf("invalid MAC encoding")}
			return
		}
		accepted := VerifyPairingHMAC([]byte(code), nonce, receivedMAC)
		resultPayload, _ := json.Marshal(PairResultPayload{Accepted: accepted})
		_ = enc.Encode(Message{Action: ActionPairResult, Payload: resultPayload})

		if !accepted {
			resultCh <- PairResult{PeerID: peerID, Err: fmt.Errorf("code mismatch")}
			return
		}

		// Add to trust store
		_ = ts.Add(peerID, TrustedPeer{
			InstanceID: peerID,
			Label:      respPayload.Label,
			PairedAt:   time.Now(),
		})
		pl.SetTrusted(peerID, true, respPayload.Label)

		resultCh <- PairResult{PeerID: peerID, Label: respPayload.Label, Success: true}
	}()

	return code, resultCh, nil
}

// handlePairStream handles incoming pairing requests from remote peers
func (s *Service) handlePairStream(stream network.Stream) {
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(pairTimeout))

	dec := json.NewDecoder(stream)
	enc := json.NewEncoder(stream)

	var msg Message
	if err := dec.Decode(&msg); err != nil {
		log.Println("pair stream: could not read request:", err)
		return
	}
	if msg.Action != ActionPairRequest {
		return
	}

	var req PairRequestPayload
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return
	}

	// Decode the nonce sent by the initiator
	nonce, err := base64.StdEncoding.DecodeString(req.Nonce)
	if err != nil || len(nonce) != 32 {
		log.Println("pair stream: invalid nonce")
		return
	}

	s.mu.Lock()
	handler := s.onPairRequest
	s.mu.Unlock()
	if handler == nil {
		// No UI handler registered — reject silently
		return
	}

	peerID := stream.Conn().RemotePeer().String()
	code, accepted := handler(peerID, req.Label)
	if !accepted {
		return
	}

	// ATTN: compute HMAC(code, nonce) instead of sending the code in plaintext.
	// The initiator holds the same code and verifies the MAC in constant time.
	mac := ComputePairingHMAC([]byte(code), nonce)
	respPayload, _ := json.Marshal(PairResponsePayload{
		MAC:   base64.StdEncoding.EncodeToString(mac),
		Label: localLabel(s.host),
	})
	if err := enc.Encode(Message{Action: ActionPairResponse, Payload: respPayload}); err != nil {
		return
	}

	// Wait for result from initiator
	var result Message
	if err := dec.Decode(&result); err != nil {
		return
	}

	var resultPayload PairResultPayload
	if err := json.Unmarshal(result.Payload, &resultPayload); err != nil {
		return
	}

	if resultPayload.Accepted {
		_ = s.trustStore.Add(peerID, TrustedPeer{
			InstanceID: peerID,
			Label:      req.Label,
			PairedAt:   time.Now(),
		})
		s.peerList.SetTrusted(peerID, true, req.Label)
		log.Printf("Paired with %s (%s)", req.Label, peerID)
	}
}

// SyncWithPeer initiates a vault sync session with a trusted peer.
// Returns the sync result describing what was transferred.
func (s *Service) SyncWithPeer(ctx context.Context, peerID string) (*SyncResult, error) {
	s.mu.Lock()
	h := s.host
	storage := s.config.Storage
	s.mu.Unlock()

	if h == nil {
		return nil, fmt.Errorf("sync service is not running")
	}
	if storage == nil {
		return nil, fmt.Errorf("storage not available")
	}

	// ATTN: verify the peer is trusted before initiating sync — mirrors the
	// check on the responder side in handleSyncStream.
	if !s.trustStore.IsTrusted(peerID) {
		return nil, fmt.Errorf("peer %s is not trusted", peerID)
	}

	pid, err := peer.Decode(peerID)
	if err != nil {
		return nil, fmt.Errorf("invalid peer ID: %w", err)
	}

	stream, err := h.NewStream(ctx, pid, SyncProtocol)
	if err != nil {
		return nil, fmt.Errorf("could not connect: %w", err)
	}
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(5 * time.Minute))

	enc := json.NewEncoder(stream)
	dec := json.NewDecoder(stream)

	// Load local catalogue
	localCatalogue, err := loadCatalogue(storage)
	if err != nil {
		return nil, fmt.Errorf("could not load catalogue: %w", err)
	}

	// Send our catalogue
	catPayload, _ := json.Marshal(CataloguePayload{VaultCatalogue: localCatalogue})
	if err := enc.Encode(Message{Action: ActionCatalogue, Payload: catPayload}); err != nil {
		return nil, fmt.Errorf("could not send catalogue: %w", err)
	}

	// Receive remote catalogue
	var remoteMsg Message
	if err := dec.Decode(&remoteMsg); err != nil {
		return nil, fmt.Errorf("could not receive catalogue: %w", err)
	}
	if remoteMsg.Action != ActionCatalogue {
		return nil, fmt.Errorf("expected catalogue, got %s", remoteMsg.Action)
	}
	var remoteCat CataloguePayload
	if err := json.Unmarshal(remoteMsg.Payload, &remoteCat); err != nil {
		return nil, fmt.Errorf("invalid catalogue: %w", err)
	}

	// Negotiate — in Strict mode, reject vaults with mismatched key fingerprints
	var negotiateOpts []NegotiateOption
	if s.config.SyncMode == usm.SyncModeStrict {
		negotiateOpts = append(negotiateOpts, WithStrictMode())
	}
	plan := Negotiate(localCatalogue, remoteCat.VaultCatalogue, negotiateOpts...)

	// Send plan
	planPayload, _ := json.Marshal(plan)
	if err := enc.Encode(Message{Action: ActionSyncPlan, Payload: planPayload}); err != nil {
		return nil, fmt.Errorf("could not send plan: %w", err)
	}

	result := &SyncResult{}

	// Execute: push local vaults
	for _, t := range plan.Auto {
		if t.Direction == SyncDirectionPush {
			if err := sendVault(enc, storage, t.VaultName); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("push %s: %v", t.VaultName, err))
				continue
			}
			result.Transfers = append(result.Transfers, t)
		}
	}

	// Execute: pull remote vaults
	for _, t := range plan.Auto {
		if t.Direction == SyncDirectionPull {
			if err := receiveVault(dec, storage, t.VaultName); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("pull %s: %v", t.VaultName, err))
				continue
			}
			result.Transfers = append(result.Transfers, t)
		}
	}

	// Complete
	_ = enc.Encode(Message{Action: ActionSyncComplete})

	// Update last sync time for this peer
	_ = s.trustStore.UpdateLastSync(peerID, time.Now())

	return result, nil
}

// handleSyncStream handles incoming sync requests from trusted peers
func (s *Service) handleSyncStream(stream network.Stream) {
	defer stream.Close()
	_ = stream.SetDeadline(time.Now().Add(5 * time.Minute))

	peerID := stream.Conn().RemotePeer().String()
	if !s.trustStore.IsTrusted(peerID) {
		log.Printf("sync: rejected untrusted peer %s", peerID)
		return
	}

	s.mu.Lock()
	storage := s.config.Storage
	notify := s.onIncomingSync
	s.mu.Unlock()
	if storage == nil {
		return
	}

	// ATTN: notify the UI so it can lock the interface while files change on disk.
	// The callback blocks until the UI is ready, then returns a done function
	// that we must call when the sync finishes.
	var done func(err error)
	if notify != nil {
		done = notify(peerID)
	}
	var syncErr error
	defer func() {
		if done != nil {
			done(syncErr)
		}
	}()

	enc := json.NewEncoder(stream)
	dec := json.NewDecoder(stream)

	// Receive initiator's catalogue
	var catMsg Message
	if err := dec.Decode(&catMsg); err != nil || catMsg.Action != ActionCatalogue {
		syncErr = fmt.Errorf("could not receive catalogue")
		return
	}
	var remoteCat CataloguePayload
	if err := json.Unmarshal(catMsg.Payload, &remoteCat); err != nil {
		syncErr = fmt.Errorf("invalid catalogue")
		return
	}

	// Send our catalogue
	localCatalogue, err := loadCatalogue(storage)
	if err != nil {
		syncErr = fmt.Errorf("could not load catalogue: %w", err)
		log.Printf("sync: %v", syncErr)
		return
	}
	catPayload, _ := json.Marshal(CataloguePayload{VaultCatalogue: localCatalogue})
	if err := enc.Encode(Message{Action: ActionCatalogue, Payload: catPayload}); err != nil {
		syncErr = fmt.Errorf("could not send catalogue")
		return
	}

	// ATTN: independently compute our own plan before receiving the initiator's.
	// This prevents a malicious paired peer from crafting a plan that triggers
	// transfers the responder's own negotiation would not allow.
	var negotiateOpts []NegotiateOption
	if s.config.SyncMode == usm.SyncModeStrict {
		negotiateOpts = append(negotiateOpts, WithStrictMode())
	}
	localPlan := Negotiate(localCatalogue, remoteCat.VaultCatalogue, negotiateOpts...)

	// Receive plan from initiator
	var planMsg Message
	if err := dec.Decode(&planMsg); err != nil || planMsg.Action != ActionSyncPlan {
		syncErr = fmt.Errorf("could not receive plan")
		return
	}
	var plan SyncPlanPayload
	if err := json.Unmarshal(planMsg.Payload, &plan); err != nil {
		syncErr = fmt.Errorf("invalid plan")
		return
	}

	// Verify the initiator's plan matches our independently computed plan
	if err := VerifyPlanConsistency(plan, localPlan); err != nil {
		syncErr = fmt.Errorf("plan verification failed: %w", err)
		log.Printf("sync: %v", syncErr)
		return
	}

	// Execute: receive pushed vaults (initiator pushes, we receive)
	for _, t := range plan.Auto {
		if t.Direction == SyncDirectionPush {
			if err := receiveVault(dec, storage, t.VaultName); err != nil {
				log.Printf("sync: receive %s failed: %v", t.VaultName, err)
			}
		}
	}

	// Execute: send pulled vaults (initiator pulls, we send)
	for _, t := range plan.Auto {
		if t.Direction == SyncDirectionPull {
			if err := sendVault(enc, storage, t.VaultName); err != nil {
				log.Printf("sync: send %s failed: %v", t.VaultName, err)
			}
		}
	}

	// Wait for completion
	var completeMsg Message
	_ = dec.Decode(&completeMsg)

	_ = s.trustStore.UpdateLastSync(peerID, time.Now())
	log.Printf("sync: completed with %s", peerID)
}

// loadCatalogue reads the vault catalogue from storage
func loadCatalogue(storage usm.Storage) (map[string]*usm.VaultEntry, error) {
	appState, err := storage.LoadAppState()
	if err != nil {
		return nil, err
	}
	if appState.VaultCatalogue == nil {
		return make(map[string]*usm.VaultEntry), nil
	}
	return appState.VaultCatalogue, nil
}

// sendVault sends all files in a vault directory over the stream
func sendVault(enc *json.Encoder, storage usm.Storage, vaultName string) error {
	vaultDir := filepath.Join(storage.Root(), "storage", vaultName)
	entries, err := os.ReadDir(vaultDir)
	if err != nil {
		return fmt.Errorf("could not read vault dir: %w", err)
	}

	// Signal start
	beginPayload, _ := json.Marshal(TransferFilePayload{VaultName: vaultName})
	if err := enc.Encode(Message{Action: ActionTransferBegin, Payload: beginPayload}); err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // skip staging/backup subdirs
		}
		data, err := os.ReadFile(filepath.Join(vaultDir, entry.Name())) //nolint:gosec // path from trusted storage root
		if err != nil {
			return fmt.Errorf("could not read %s: %w", entry.Name(), err)
		}
		filePayload, _ := json.Marshal(TransferFilePayload{
			VaultName: vaultName,
			FileName:  entry.Name(),
			Data:      base64.StdEncoding.EncodeToString(data),
		})
		if err := enc.Encode(Message{Action: ActionTransferFile, Payload: filePayload}); err != nil {
			return err
		}
	}

	endPayload, _ := json.Marshal(TransferFilePayload{VaultName: vaultName})
	return enc.Encode(Message{Action: ActionTransferEnd, Payload: endPayload})
}

// receiveVault receives vault files from the stream into a staging directory
// and atomically commits them
func receiveVault(dec *json.Decoder, storage usm.Storage, vaultName string) error {
	// ATTN: validate vault name from remote peer to prevent path traversal
	if err := validateSyncName(vaultName); err != nil {
		return fmt.Errorf("invalid vault name: %w", err)
	}

	vaultDir := filepath.Join(storage.Root(), "storage", vaultName)

	// Ensure vault dir exists
	if err := os.MkdirAll(vaultDir, 0o700); err != nil {
		return fmt.Errorf("could not create vault dir: %w", err)
	}

	staging, err := PrepareStaging(vaultDir)
	if err != nil {
		return err
	}

	// Read transfer_begin
	var beginMsg Message
	if err := dec.Decode(&beginMsg); err != nil {
		return fmt.Errorf("expected transfer_begin: %w", err)
	}
	if beginMsg.Action != ActionTransferBegin {
		return fmt.Errorf("expected transfer_begin, got %s", beginMsg.Action)
	}

	// Read files until transfer_end
	for {
		var msg Message
		if err := dec.Decode(&msg); err != nil {
			_ = os.RemoveAll(staging)
			return fmt.Errorf("stream read error: %w", err)
		}

		if msg.Action == ActionTransferEnd {
			break
		}
		if msg.Action != ActionTransferFile {
			_ = os.RemoveAll(staging)
			return fmt.Errorf("unexpected action during transfer: %s", msg.Action)
		}

		var fp TransferFilePayload
		if err := json.Unmarshal(msg.Payload, &fp); err != nil {
			_ = os.RemoveAll(staging)
			return fmt.Errorf("invalid file payload: %w", err)
		}

		// ATTN: validate file name from remote peer to prevent path traversal
		if err := validateSyncName(fp.FileName); err != nil {
			_ = os.RemoveAll(staging)
			return fmt.Errorf("invalid file name: %w", err)
		}

		data, err := base64.StdEncoding.DecodeString(fp.Data)
		if err != nil {
			_ = os.RemoveAll(staging)
			return fmt.Errorf("invalid base64 in %s: %w", fp.FileName, err)
		}

		// ATTN: enforce file size limit to prevent memory exhaustion
		if len(data) > maxTransferFileSize {
			_ = os.RemoveAll(staging)
			return fmt.Errorf("file %s exceeds maximum size (%d > %d)", fp.FileName, len(data), maxTransferFileSize)
		}

		if err := os.WriteFile(filepath.Join(staging, fp.FileName), data, 0o600); err != nil {
			_ = os.RemoveAll(staging)
			return fmt.Errorf("could not write %s: %w", fp.FileName, err)
		}
	}

	// Atomic commit: backup existing vault, swap in staged files
	return CommitStaging(vaultDir)
}

// localLabel returns a human-readable name for this instance
func localLabel(h host.Host) string {
	name, err := os.Hostname()
	if err != nil {
		pid := h.ID().String()
		if len(pid) > 12 {
			return pid[:12]
		}
		return pid
	}
	return name
}
