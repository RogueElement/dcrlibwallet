package dcrlibwallet

import (
	"context"
	"os"
	"path/filepath"

	"github.com/asdine/storm"
	"github.com/decred/dcrwallet/errors/v2"
	w "github.com/decred/dcrwallet/wallet/v3"
	"github.com/kevinburke/nacl"
	"github.com/kevinburke/nacl/secretbox"
	"github.com/raedahgroup/dcrlibwallet/spv"
	"golang.org/x/crypto/bcrypt"
)

const (
	logFileName   = "dcrlibwallet.log"
	walletsDbName = "wallets.db"

	walletsMetadataBucketName    = "metadata"
	walletstartupPassphraseField = "startup-passphrase"
)

func (mw *MultiWallet) batchDbTransaction(dbOp func(node storm.Node) error) (err error) {
	dbTx, err := mw.db.Begin(true)
	if err != nil {
		return err
	}

	// Commit or rollback the transaction after f returns or panics.  Do not
	// recover from the panic to keep the original stack trace intact.
	panicked := true
	defer func() {
		if panicked || err != nil {
			dbTx.Rollback()
			return
		}

		err = dbTx.Commit()
	}()

	err = dbOp(dbTx)
	panicked = false
	return err
}

func (mw *MultiWallet) loadWalletTemporarily(ctx context.Context, walletDataDir, walletPublicPass string,
	onLoaded func(*w.Wallet) error) error {

	if walletPublicPass == "" {
		walletPublicPass = w.InsecurePubPassphrase
	}

	// initialize the wallet loader
	walletLoader := initWalletLoader(mw.chainParams, walletDataDir, mw.dbDriver)

	// open the wallet to get ready for temporary use
	wallet, err := walletLoader.OpenExistingWallet(ctx, []byte(walletPublicPass))
	if err != nil {
		return translateError(err)
	}

	// unload wallet after temporary use
	defer walletLoader.UnloadWallet()

	if onLoaded != nil {
		return onLoaded(wallet)
	}

	return nil
}

func (mw *MultiWallet) markWalletAsDiscoveredAccounts(walletID int) error {
	wallet := mw.WalletWithID(walletID)
	if wallet == nil {
		return errors.New(ErrNotExist)
	}

	log.Infof("Set discovered accounts = true for wallet %d", wallet.ID)
	wallet.HasDiscoveredAccounts = true
	err := mw.db.Save(wallet)
	if err != nil {
		return err
	}

	return nil
}

func (mw *MultiWallet) setNetworkBackend(syncer *spv.Syncer) {
	for walletID, wallet := range mw.wallets {
		if wallet.WalletOpened() {
			walletBackend := &spv.WalletBackend{
				Syncer:   syncer,
				WalletID: walletID,
			}
			wallet.internal.SetNetworkBackend(walletBackend)
		}
	}
}

// RootDirFileSizeInBytes returns the total directory size of
// multiwallet's root directory in bytes.
func (mw *MultiWallet) RootDirFileSizeInBytes() (int64, error) {
	var size int64
	err := filepath.Walk(mw.rootDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}

// encryptWalletSeed encrypts the seed with secretbox.EasySeal.
func encryptWalletSeed(pass []byte, seed string) ([]byte, error) {
	hash, err := bcrypt.GenerateFromPassword(pass, bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	key, err := nacl.Load(string(hash))
	if err != nil {
		return nil, err
	}

	encryptedSeed := secretbox.EasySeal([]byte(seed), key)

	return encryptedSeed, nil
}

func decryptWalletSeed(pass []byte, encryptedSeed []byte) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(pass, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	key, err := nacl.Load(string(hash))
	if err != nil {
		return "", err
	}

	decryptedSeed, err := secretbox.EasyOpen(encryptedSeed, key)
	if err != nil {
		return "", err
	}

	return string(decryptedSeed), nil
}
