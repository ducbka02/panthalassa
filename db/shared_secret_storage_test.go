package db

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ed25519"
	"time"
)

func TestBoltSharedSecretStorage_Put(t *testing.T) {

	// setup
	db := createDB()
	km := createKeyManager()
	storage := NewBoltSharedSecretStorage(db, km)
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.Nil(t, err)

	baseID := make([]byte, 32)
	_, err = rand.Read(baseID)
	require.Nil(t, err)

	// persist shared secret
	require.Nil(t, storage.Put(pub, SharedSecret{
		X3dhSS: [32]byte{1, 2},
		ID:     []byte("shared-secret-id"),
		BaseID: baseID,
	}))

	// fetch shared secret
	sharedSec, err := storage.Get(pub[:], baseID)
	require.Nil(t, err)
	require.NotNil(t, sharedSec)

	require.Equal(t, []byte("shared-secret-id"), sharedSec.ID)

}

func TestBoltSharedSecretStorage_Accept(t *testing.T) {

	// setup
	db := createDB()
	km := createKeyManager()
	storage := NewBoltSharedSecretStorage(db, km)
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.Nil(t, err)

	baseID := make([]byte, 32)
	_, err = rand.Read(baseID)
	require.Nil(t, err)

	// persist shared secret
	require.Nil(t, storage.Put(pub, SharedSecret{
		X3dhSS: [32]byte{1, 2},
		ID:     []byte("shared-secret-id"),
		BaseID: baseID,
	}))

	// accept shared secret
	require.Nil(t, storage.Accept(pub, &SharedSecret{
		X3dhSS: [32]byte{1, 2},
		ID:     []byte("shared-secret-id"),
		BaseID: baseID,
	}))

	// fetch shared secret
	ss, err := storage.Get(pub, baseID)
	require.Nil(t, err)
	require.NotNil(t, ss)
	require.True(t, ss.Accepted)

}

func TestBoltSharedSecretStorage_SecretForChatInitMsg(t *testing.T) {

	// setup
	db := createDB()
	km := createKeyManager()
	storage := NewBoltSharedSecretStorage(db, km)
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.Nil(t, err)

	baseID := make([]byte, 32)
	_, err = rand.Read(baseID)
	require.Nil(t, err)

	// persist shared secret
	require.Nil(t, storage.Put(pub, SharedSecret{
		X3dhSS:       [32]byte{1, 2},
		ID:           []byte("shared-secret-id"),
		IDInitParams: []byte("chat-init-params-id"),
		BaseID:       baseID,
	}))

	ss, err := storage.SecretForChatInitMsg(pub, []byte("chat-init-params-id"))
	require.Nil(t, err)
	require.NotNil(t, ss)

	require.Equal(t, [32]byte{1, 2}, ss.X3dhSS)

}

func TestBoltSharedSecretStorage_Get(t *testing.T) {

	// setup
	db := createDB()
	km := createKeyManager()
	storage := NewBoltSharedSecretStorage(db, km)
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.Nil(t, err)

	baseID := make([]byte, 32)
	_, err = rand.Read(baseID)
	require.Nil(t, err)

	// persist shared secret
	require.Nil(t, storage.Put(pub, SharedSecret{
		X3dhSS: [32]byte{1, 2},
		ID:     []byte("shared-secret-id"),
		BaseID: baseID,
	}))

	ss, err := storage.Get(pub, baseID)
	require.Nil(t, err)
	require.NotNil(t, ss)

	require.Equal(t, [32]byte{1, 2}, ss.X3dhSS)

}

func TestBoltSharedSecretStorage_GetYoungest(t *testing.T) {

	// setup
	db := createDB()
	km := createKeyManager()
	storage := NewBoltSharedSecretStorage(db, km)
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.Nil(t, err)

	baseID := make([]byte, 32)
	_, err = rand.Read(baseID)
	require.Nil(t, err)

	// persist first shared secret
	require.Nil(t, storage.Put(pub, SharedSecret{
		X3dhSS:    [32]byte{},
		ID:        []byte("id-one"),
		CreatedAt: time.Now().Truncate(time.Minute),
		BaseID:    baseID,
	}))

	baseID = make([]byte, 32)
	_, err = rand.Read(baseID)
	require.Nil(t, err)

	// persist first shared secret
	require.Nil(t, storage.Put(pub, SharedSecret{
		X3dhSS:    [32]byte{1, 2},
		ID:        []byte("id-two"),
		CreatedAt: time.Now(),
		BaseID:    baseID,
	}))

	ss, err := storage.GetYoungest(pub)
	require.Nil(t, err)
	require.NotNil(t, ss)

	require.Equal(t, [32]byte{1, 2}, ss.X3dhSS)

}

func TestBoltSharedSecretStorage_HasAny(t *testing.T) {

	// setup
	db := createDB()
	km := createKeyManager()
	storage := NewBoltSharedSecretStorage(db, km)
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.Nil(t, err)

	// should be false since we didn't persist a shared secret
	has, err := storage.HasAny(pub)
	require.Nil(t, err)
	require.False(t, has)

	// persist first shared secret
	require.Nil(t, storage.Put(pub, SharedSecret{
		X3dhSS:    [32]byte{},
		ID:        []byte("id-one"),
		CreatedAt: time.Now().Truncate(time.Minute),
		BaseID:    make([]byte, 32),
	}))

	// must be true since we persisted a shared secret
	has, err = storage.HasAny(pub)
	require.Nil(t, err)
	require.True(t, has)

}
