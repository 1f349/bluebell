package peers

import (
	"context"
	"crypto/x509"
	"github.com/1f349/bluebell/logger"
	"github.com/1f349/cache"
	"github.com/spf13/afero"
	"net/netip"
	"path/filepath"
	"time"
)

const cacheDuration = 5 * time.Hour
const cacheRefreshLoop = 4 * time.Hour

func New(fs afero.Fs) (*Peers, error) {
	ctx, cancel := context.WithCancel(context.Background())

	p := &Peers{
		ctxCancel: cancel,
		dir:       fs,
	}
	go p.Load(ctx)
	return p, nil
}

type Peers struct {
	ctxCancel context.CancelFunc
	dir       afero.Fs
	certCache cache.Cache[netip.Addr, *x509.Certificate]
}

func (p *Peers) Close() {
	p.ctxCancel()
}

func (p *Peers) Load(ctx context.Context) {
	for {
		select {
		case <-time.After(cacheDuration):
			err := p.internalLoad()
			if err != nil {
				logger.Logger.Error("failed to load new peers", "err", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *Peers) internalLoad() error {
	dirEntries, err := afero.ReadDir(p.dir, ".")
	if err != nil {
		return err
	}

	for _, i := range dirEntries {
		// ignore a file without the extension ".pem"
		if filepath.Ext(i.Name()) != ".pem" {
			continue
		}

		err := p.readSingleCert(i.Name())
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Peers) readSingleCert(name string) error {
	certData, err := afero.ReadFile(p.dir, name)
	if err != nil {
		return err
	}

	certificate, err := x509.ParseCertificate(certData)
	if err != nil {
		return err
	}

	for _, ip := range certificate.IPAddresses {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}
		p.certCache.Set(addr, certificate, 1*time.Hour)
	}

	return nil
}
