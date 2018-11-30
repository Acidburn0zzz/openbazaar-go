package ipfs

import (
	"context"
	"gx/ipfs/QmPSQnBKM9g7BaUcZCvswUJVscQ1ipjmwxN5PXCjkp9EQ7/go-cid"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/ipfs/go-ipfs/core/coreapi"
	"github.com/ipfs/go-ipfs/core/corerepo"

	"gx/ipfs/QmSei8kFMfqdJq7Q68d2LMnHbTWKKg2daA29ezUYFAUNgc/go-merkledag"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/core/coreunix"
	_ "github.com/ipfs/go-ipfs/core/mock"
)

// Recursively add a directory to IPFS and return the root hash
func AddDirectory(n *core.IpfsNode, root string) (rootHash string, err error) {
	ctx := context.Background()
	s := strings.Split(root, "/")
	dirName := s[len(s)-1]
	h, err := coreunix.AddR(n, root)

	i, err := cid.Decode(h)
	if err != nil {
		return "", err
	}
	api := coreapi.NewCoreAPI(n)
	_, err = corerepo.Pin(n, api, ctx, []string{i.String()}, true)
	if err != nil {
		return "", err
	}
	dag := merkledag.NewDAGService(n.Blocks)
	m := make(map[string]bool)

	m[i.String()] = true
	for {
		if len(m) == 0 {
			break
		}
		for k := range m {
			c, err := cid.Decode(k)
			if err != nil {
				return "", err
			}
			links, err := dag.GetLinks(ctx, c)
			if err != nil {
				return "", err
			}
			delete(m, k)
			for _, link := range links {
				if link.Name == dirName {
					return link.Cid.String(), nil
				}
				m[link.Cid.String()] = true
			}
		}
	}
	return i.String(), nil
}

func AddFile(n *core.IpfsNode, file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", nil
	}
	return coreunix.Add(n, f)
}

func GetHashOfFile(n *core.IpfsNode, fpath string) (string, error) {
	return AddFile(n, fpath)
}

func GetHash(n *core.IpfsNode, reader io.Reader) (string, error) {
	f, err := ioutil.TempFile("", strconv.Itoa(rand.Int()))
	if err != nil {
		return "", err
	}
	b, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	f.Write(b)
	defer f.Close()
	return GetHashOfFile(n, f.Name())
}
