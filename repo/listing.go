package repo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	cid "gx/ipfs/QmTbxNB1NwDesLmKTscr4udL2tVP7MaxvXnD1D9yX7g3PN/go-cid"
	mh "gx/ipfs/QmerPMzPk1mJVowm8KgmoknWa4yCYvvugMPsgWmDNUvDLW/go-multihash"

	"github.com/OpenBazaar/jsonpb"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/golang/protobuf/proto"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/gosimple/slug"
	"github.com/ipfs/go-ipfs/core"
	"github.com/microcosm-cc/bluemonday"

	"github.com/OpenBazaar/openbazaar-go/ipfs"
	"github.com/OpenBazaar/openbazaar-go/pb"
	"github.com/OpenBazaar/openbazaar-go/util"
)

const (
	// ListingVersion - current listing version
	ListingVersion = 5
	// TitleMaxCharacters - max size for title
	TitleMaxCharacters = 140
	// ShortDescriptionLength - min length for description
	ShortDescriptionLength = 160
	// DescriptionMaxCharacters - max length for description
	DescriptionMaxCharacters = 50000
	// MaxTags - max permitted tags
	MaxTags = 10
	// MaxCategories - max permitted categories
	MaxCategories = 10
	// MaxListItems - max items in a listing
	MaxListItems = 30
	// FilenameMaxCharacters - max filename size
	FilenameMaxCharacters = 255
	// CodeMaxCharacters - max chars for a code
	CodeMaxCharacters = 20
	// WordMaxCharacters - max chars for word
	WordMaxCharacters = 40
	// SentenceMaxCharacters - max chars for sentence
	SentenceMaxCharacters = 70
	// CouponTitleMaxCharacters - max length of a coupon title
	CouponTitleMaxCharacters = 70
	// PolicyMaxCharacters - max length for policy
	PolicyMaxCharacters = 10000
	// AboutMaxCharacters - max length for about
	AboutMaxCharacters = 10000
	// URLMaxCharacters - max length for URL
	URLMaxCharacters = 2000
	// MaxCountryCodes - max country codes
	MaxCountryCodes = 255
	// EscrowTimeout - escrow timeout in hours
	EscrowTimeout = 1080
	// SlugBuffer - buffer size for slug
	SlugBuffer = 5
	// PriceModifierMin - min price modifier
	PriceModifierMin = -99.99
	// PriceModifierMax = max price modifier
	PriceModifierMax = 1000.00
)

type option struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type shippingOption struct {
	Name    string `json:"name"`
	Service string `json:"service"`
}

type Item struct {
	ListingHash    string         `json:"listingHash"`
	Quantity       uint64         `json:"quantity"`
	Options        []option       `json:"options"`
	Shipping       shippingOption `json:"shipping"`
	Memo           string         `json:"memo"`
	Coupons        []string       `json:"coupons"`
	PaymentAddress string         `json:"paymentAddress"`
}

// PurchaseData - record purchase data
type PurchaseData struct {
	ShipTo               string  `json:"shipTo"`
	Address              string  `json:"address"`
	City                 string  `json:"city"`
	State                string  `json:"state"`
	PostalCode           string  `json:"postalCode"`
	CountryCode          string  `json:"countryCode"`
	AddressNotes         string  `json:"addressNotes"`
	Moderator            string  `json:"moderator"`
	Items                []Item  `json:"items"`
	AlternateContactInfo string  `json:"alternateContactInfo"`
	RefundAddress        *string `json:"refundAddress"` //optional, can be left out of json
	PaymentCoin          string  `json:"paymentCoin"`
}

// NewListingFromProtobuf - return Listing from pb.Listing
func NewListingFromProtobuf(l *pb.Listing) (*Listing, error) {
	var vendorInfo *PeerInfo
	var err error
	if l.VendorID != nil {
		vendorInfo, err = NewPeerInfoFromProtobuf(l.VendorID)
		if err != nil {
			return nil, fmt.Errorf("new peer info: %s", err)
		}
	}

	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: false,
		Indent:       "    ",
		OrigName:     false,
	}

	out, err := m.MarshalToString(l)
	if err != nil {
		return nil, err
	}
	listing0 := Listing{
		Slug:               l.Slug,
		TermsAndConditions: l.TermsAndConditions,
		RefundPolicy:       l.RefundPolicy,
		Vendor:             vendorInfo,
		ListingBytes:       []byte(out),
		ListingVersion:     l.Metadata.Version,
		ProtoListing:       l,
	}
	return &listing0, nil
}

// CreateListing will create a pb Listing
func CreateListing(r []byte, isTestnet bool, dstore *Datastore, repoPath string) (Listing, error) {
	ld := new(pb.Listing)
	//data := make([]byte, 100000)
	//n, err := r.Read(data)
	//if err != nil && err != io.EOF {
	//	return Listing{}, err
	//}
	err := jsonpb.UnmarshalString(string(r), ld)
	log.Info("after unmarshalling , listing : ")
	log.Info(*ld)
	if err != nil {
		return Listing{}, err
	}
	slug := ld.Slug
	exists, err := listingExists(slug, repoPath, isTestnet)
	log.Info("after listingExists  : ", exists, "  err  ", err)
	if err != nil {
		return Listing{}, err
	}
	if exists {
		return Listing{}, ErrListingAlreadyExists
	}
	log.Info("slug before :  ", slug)
	if slug == "" {
		slug, err = GenerateSlug(ld.Item.Title, repoPath, isTestnet, dstore)
		log.Info("after gen slug  : ", slug, "  err  ", err)
		if err != nil {
			return Listing{}, err
		}
		ld.Slug = slug
	}
	retListing, err := NewListingFromProtobuf(ld)
	log.Info("after new listing from pb   :  err  ", err)
	return *retListing, err
}

// UpdateListing will update a pb Listing
func UpdateListing(r []byte, isTestnet bool, dstore *Datastore, repoPath string) (Listing, error) {
	ld := new(pb.Listing)
	//data := make([]byte, 100000)
	//n, err := r.Read(data)
	//if err != nil && err != io.EOF {
	//	return Listing{}, err
	//}
	err := jsonpb.UnmarshalString(string(r), ld)
	if err != nil {
		return Listing{}, err
	}
	slug := ld.Slug
	exists, err := listingExists(slug, repoPath, isTestnet)
	if err != nil {
		return Listing{}, err
	}
	if !exists {
		return Listing{}, ErrListingDoesNotExist
	}
	retListing, err := NewListingFromProtobuf(ld)
	return *retListing, err
}

// GenerateSlug - slugify the title of the listing
func GenerateSlug(title, repoPath string, isTestnet bool, dStore *Datastore) (string, error) {
	title = strings.Replace(title, "/", "", -1)
	counter := 1
	slugBase := CreateSlugFor(title)
	slugToTry := slugBase
	for {
		_, err := GetListingFromSlug(slugToTry, repoPath, isTestnet, dStore)
		if os.IsNotExist(err) {
			return slugToTry, nil
		} else if err != nil {
			return "", err
		}
		slugToTry = slugBase + strconv.Itoa(counter)
		counter++
	}
}

// GetListingFromSlug - fetch listing for the specified slug
func GetListingFromSlug(slug, repoPath string, isTestnet bool, dStore *Datastore) (*pb.SignedListing, error) {
	repoPath, err := GetRepoPath(isTestnet, repoPath)
	if err != nil {
		return nil, err
	}
	// Read listing file
	listingPath := path.Join(repoPath, "root", "listings", slug+".json")
	file, err := ioutil.ReadFile(listingPath)
	if err != nil {
		return nil, err
	}

	// Unmarshal listing
	sl := new(pb.SignedListing)
	err = jsonpb.UnmarshalString(string(file), sl)
	if err != nil {
		return nil, err
	}

	// Get the listing inventory
	inventory, err := (*dStore).Inventory().Get(slug)
	if err != nil {
		return nil, err
	}

	// Build the inventory list
	for variant, count := range inventory {
		for i, s := range sl.Listing.Item.Skus {
			if variant == i {
				s.Quantity = count
				break
			}
		}
	}
	return sl, nil
}

func listingExists(slug, repoPath string, isTestnet bool) (bool, error) {
	if slug == "" {
		return false, nil
	}
	fPath, err := GetPathForListingSlug(slug, repoPath, isTestnet)
	if err != nil {
		return false, err
	}
	_, ferr := os.Stat(fPath)
	if slug == "" {
		return false, nil
	}
	if os.IsNotExist(ferr) {
		return false, nil
	}
	if ferr != nil {
		return false, ferr
	}
	return true, nil
}

func GetPathForListingSlug(slug, repoPath string, isTestnet bool) (string, error) {
	repoPath, err := GetRepoPath(isTestnet, repoPath)
	if err != nil {
		return "", err
	}
	return path.Join(repoPath, "root", "listings", slug+".json"), nil
}

func ToHtmlEntities(str string) string {
	var rx = regexp.MustCompile(util.EmojiPattern)
	return rx.ReplaceAllStringFunc(str, func(s string) string {
		r, _ := utf8.DecodeRuneInString(s)
		html := fmt.Sprintf(`&#x%X;`, r)
		return html
	})
}

// CreateSlugFor Create a slug from a multi-lang string
func CreateSlugFor(slugName string) string {
	l := SentenceMaxCharacters - SlugBuffer

	slugName = ToHtmlEntities(slugName)

	slug := slug.Make(slugName)
	if len(slug) < SentenceMaxCharacters-SlugBuffer {
		l = len(slug)
	}
	return slug[:l]
}

// IndividualListingContainer -
type IndividualListingContainer struct {
	Listing `json:"listing"`
}

// Listing represents a trade offer which can be accepted by another
// party on the OpenBazaar network
type Listing struct {
	Slug               string //`json:"slug"`
	TermsAndConditions string //`json:"termsAndConditions"`
	RefundPolicy       string //`json:"refundPolicy"`

	Vendor   *PeerInfo       //`json:"vendorID"`
	Metadata ListingMetadata //`json:"metadata"`
	//Hash               string    `json:"hash"`
	//Title              string    `json:"title"`
	//Categories         []string  `json:"categories"`
	//NSFW               bool      `json:"nsfw"`
	//ContractType       string    `json:"contractType"`
	//Description        string    `json:"description"`
	//Thumbnail          thumbnail `json:"thumbnail"`
	//Price              price     `json:"price"`
	//ShipsTo            []string  `json:"shipsTo"`
	//FreeShipping       []string  `json:"freeShipping"`
	//Language           string    `json:"language"`
	//AverageRating      float32   `json:"averageRating"`
	//RatingCount        uint32    `json:"ratingCount"`
	//ModeratorIDs       []string  `json:"moderators"`
	//AcceptedCurrencies []string  `json:"acceptedCurrencies"`
	//CoinType           string    `json:"coinType"`

	ListingBytes   []byte `json:"-"`
	ListingVersion uint32 `json:"-"`

	ProtoListing *pb.Listing `json:"-"`
}

func (l *Listing) Reset()         { *l = Listing{} }
func (l *Listing) String() string { return proto.CompactTextString(l) }
func (*Listing) ProtoMessage()    {}

func (r Listing) eof() bool {
	return len(r.ListingBytes) == 0
}

func (r *Listing) readByte(n int) byte {
	// this function assumes that eof() check was done before
	return r.ListingBytes[n]
}

func (r *Listing) Read(p []byte) (n int, err error) {
	if n == len(r.ListingBytes)-1 { //r.eof() {
		//err = io.EOF
		return
	}

	//if c := cap(p); c > 0 {
	if c := len(r.ListingBytes); c > 0 {
		//fmt.Println("what is c ? ", c, "   and n : ", n)
		for n < c {
			//fmt.Println("now n is : ", n)
			p[n] = r.readByte(n)
			n++
			if r.eof() {
				break
			}
		}
	}
	return n, nil
}

type SignedListing struct {
	Hash         string `json:"hash"`
	Signature    []byte `json:"signature"`
	Listing      `json:"listing"`
	ProtoListing *pb.Listing `-`

	ProtoSignedListing *pb.SignedListing `json:"-"`
}

// ListingMetadata -
type ListingMetadata struct {
	Version uint `json:"version"`
}

// UnmarshalJSONSignedListing - unmarshal signed listing
func UnmarshalJSONSignedListing(data []byte) (SignedListing, error) {
	log.Info("in repo unmarshal json signed listing")
	var l SignedListing
	type lstg struct {
		Listing interface{} `json:"listing"`
	}
	var v lstg
	if err := json.Unmarshal(data, &l); err != nil {
		return l, fmt.Errorf("unmarshal listing: %s", err.Error())
	}
	log.Info("after unmarshalling, sl : ")
	log.Info(l)
	if err := json.Unmarshal(data, &v); err != nil {
		return l, fmt.Errorf("unmarshal listing: %s", err.Error())
	}
	//log.Info("after unmarshalling . v : ")
	//log.Info(v)
	//listingData, ok := v.(map[string]interface{})
	//if !ok {
	//	return nil, fmt.Errorf("unmarshal listing: %s", "incorrect data")
	//}
	//listing, ok := listingData["listing"]
	//if !ok {
	//	return nil, fmt.Errorf("unmarshal listing: %s", "incorrect data")
	//}
	out, _ := json.Marshal(v.Listing)
	//log.Info("lets eee out  : ")
	//log.Info(out)
	log.Info("and version  : ")
	log.Info(l.Metadata.Version)

	l.Listing.ListingBytes = out
	l.Listing.ListingVersion = uint32(l.Metadata.Version)
	pl, err := l.Listing.GetProtoListing()
	log.Info("lets see the proto listing now: ")
	log.Info(pl)
	if err != nil {
		log.Info("pl err : ", err)
	}
	l.ProtoListing = pl
	l.Listing.ProtoListing = pl
	l.Listing.Vendor, err = NewPeerInfoFromProtobuf(pl.VendorID)
	if err != nil {
		log.Info("vendor ext err : ", err)
	}

	return l, nil
}

// UnmarshalJSONListing - unmarshal listing
func UnmarshalJSONListing(data []byte) (*Listing, error) {
	l, err := UnmarshalJSONSignedListing(data)
	if err != nil {
		return nil, err
	}
	return &l.Listing, nil
}

// GetTitle - return listing title
func (l *Listing) GetTitle() (string, error) {
	type title struct {
		Item struct {
			Title string `json:"title"`
		} `json:"item"`
	}
	var t title
	err := json.Unmarshal(l.ListingBytes, &t)
	if err != nil {
		return "", err
	}
	return t.Item.Title, nil
}

// GetSlug - return listing slug
func (l *Listing) GetSlug() (string, error) {
	type slug struct {
		Slug string `json:"slug"`
	}
	var s slug
	err := json.Unmarshal(l.ListingBytes, &s)
	if err != nil {
		return "", err
	}
	return s.Slug, nil
}

// GetAcceptedCurrencies - return list of accepted currencies
func (l *Listing) GetAcceptedCurrencies() ([]string, error) {
	type acceptedCurrencies struct {
		Metadata struct {
			AcceptedCurrencies []string `json:"acceptedCurrencies"`
		} `json:"metadata"`
	}
	var a acceptedCurrencies
	err := json.Unmarshal(l.ListingBytes, &a)
	if err != nil {
		return []string{}, err
	}
	return a.Metadata.AcceptedCurrencies, nil
}

// GetContractType - return listing's contract type
func (l *Listing) GetContractType() (string, error) {
	retVal := ""
	type contractType struct {
		Metadata struct {
			ContractType string `json:"contractType"`
		} `json:"metadata"`
	}
	var ct contractType
	err := json.Unmarshal(l.ListingBytes, &ct)
	if err != nil {
		return retVal, err
	}
	return ct.Metadata.ContractType, nil
}

// GetFormat - return listing's format
func (l *Listing) GetFormat() (string, error) {
	retVal := ""
	type format struct {
		Metadata struct {
			Format string `json:"format"`
		} `json:"metadata"`
	}
	var ct format
	err := json.Unmarshal(l.ListingBytes, &ct)
	if err != nil {
		return retVal, err
	}
	return ct.Metadata.Format, nil
}

// GetPrice - return listing price
func (l *Listing) GetPrice() (CurrencyValue, error) {
	retVal := CurrencyValue{}
	contractType, err := l.GetContractType()
	if err != nil {
		return retVal, err
	}
	switch l.ListingVersion {
	case 3, 4:
		{
			if contractType == "CRYPTOCURRENCY" {
				retVal.Amount = big.NewInt(0)
				type coinType struct {
					Metadata struct {
						CoinType string `json:"coinType"`
					} `json:"metadata"`
				}
				var c coinType
				json.Unmarshal(l.ListingBytes, &c)
				retVal.Currency = &CurrencyDefinition{
					Code:         CurrencyCode(c.Metadata.CoinType),
					Divisibility: 8,
				}
			} else {
				type price struct {
					Item struct {
						Price int64 `json:"price"`
					} `json:"item"`
				}
				var p price
				json.Unmarshal(l.ListingBytes, &p)
				retVal.Amount = big.NewInt(p.Item.Price)
				type pricingCurrency struct {
					Metadata struct {
						PricingCurrency string `json:"pricingCurrency"`
					} `json:"metadata"`
				}
				var pc pricingCurrency
				json.Unmarshal(l.ListingBytes, &pc)
				retVal.Currency = &CurrencyDefinition{
					Code:         CurrencyCode(pc.Metadata.PricingCurrency),
					Divisibility: 8,
				}
			}
		}
	case 5:
		{
			type price struct {
				Item struct {
					Price struct {
						Currency struct {
							Code         string `json:"code"`
							Divisibility uint   `json:"divisibility"`
						} `json:"currency"`
						Amount string `json:"amount"`
					} `json:"price"`
				} `json:"item"`
			}
			var p price
			json.Unmarshal(l.ListingBytes, &p)
			retVal.Amount, _ = new(big.Int).SetString(p.Item.Price.Amount, 10)
			retVal.Currency = &CurrencyDefinition{
				Code:         CurrencyCode(p.Item.Price.Currency.Code),
				Divisibility: p.Item.Price.Currency.Divisibility,
			}
		}
	}
	return retVal, nil
}

// GetModerators - return listing moderators
func (l *Listing) GetModerators() ([]string, error) {
	type mods struct {
		Moderators []string `json:"moderators"`
	}
	var s mods
	err := json.Unmarshal(l.ListingBytes, &s)
	if err != nil {
		return []string{}, err
	}
	return s.Moderators, nil
}

// SetModerators - set listing moderators
func (l *Listing) SetModerators(mods []string) error {
	listing, err := l.GetProtoListing()
	if err != nil {
		return err
	}
	listing.Moderators = mods
	// TODO: set the bytes here
	return nil
}

// GetTermsAndConditions - return listing termsAndConditions
func (l *Listing) GetTermsAndConditions() (string, error) {
	type tnc struct {
		TermsAndConditions string `json:"termsAndConditions"`
	}
	var s tnc
	err := json.Unmarshal(l.ListingBytes, &s)
	if err != nil {
		return "", err
	}
	return s.TermsAndConditions, nil
}

// GetRefundPolicy - return listing refundPolicy
func (l *Listing) GetRefundPolicy() (string, error) {
	type rp struct {
		RefundPolicy string `json:"refundPolicy"`
	}
	var s rp
	err := json.Unmarshal(l.ListingBytes, &s)
	if err != nil {
		return "", err
	}
	return s.RefundPolicy, nil
}

// GetVendorID - return vendorID
func (l *Listing) GetVendorID() (*pb.ID, error) {
	if l.Vendor == nil {
		type pbID struct {
			VendorID struct {
				PeerID     string `json:"peerID"`
				BitcoinSig string `json:"bitcoinSig"`
				PubKeys    struct {
					Identity string `json:"identity"`
					Bitcoin  string `json:"bitcoin"`
				} `json:"pubkeys"`
			} `json:"vendorID"`
		}
		var s pbID
		err := json.Unmarshal(l.ListingBytes, &s)
		if err != nil {
			return nil, err
		}
		pid := &pb.ID{
			PeerID:     s.VendorID.PeerID,
			BitcoinSig: []byte(s.VendorID.BitcoinSig),
			Pubkeys: &pb.ID_Pubkeys{
				Identity: []byte(s.VendorID.PubKeys.Identity),
				Bitcoin:  []byte(s.VendorID.PubKeys.Bitcoin),
			},
		}
		log.Info("the pb id is : ", pid)
		l.Vendor, err = NewPeerInfoFromProtobuf(pid)
		if err != nil {
			log.Info("err getting peer info   : ", err)
			return nil, err
		}
	}
	return l.Vendor.Protobuf(), nil
}

// GetDescription - return item description
func (l *Listing) GetDescription() (string, error) {
	type desc struct {
		Item struct {
			Description string `json:"description"`
		} `json:"item"`
	}
	var t desc
	err := json.Unmarshal(l.ListingBytes, &t)
	if err != nil {
		return "", err
	}
	return t.Item.Description, nil
}

// GetProcessingTime - return item processing time
func (l *Listing) GetProcessingTime() (string, error) {
	type ptime struct {
		Item struct {
			ProcessingTime string `json:"processingTime"`
		} `json:"item"`
	}
	var t ptime
	err := json.Unmarshal(l.ListingBytes, &t)
	if err != nil {
		return "", err
	}
	return t.Item.ProcessingTime, nil
}

// GetNsfw - return item nstw
func (l *Listing) GetNsfw() (bool, error) {
	type nsfw struct {
		Item struct {
			Nsfw bool `json:"nsfw"`
		} `json:"item"`
	}
	var t nsfw
	err := json.Unmarshal(l.ListingBytes, &t)
	if err != nil {
		return false, err
	}
	return t.Item.Nsfw, nil
}

// GetTags - return item tags
func (l *Listing) GetTags() ([]string, error) {
	type tags struct {
		Item struct {
			Tags []string `json:"tags"`
		} `json:"item"`
	}
	var t tags
	err := json.Unmarshal(l.ListingBytes, &t)
	if err != nil {
		return nil, err
	}
	return t.Item.Tags, nil
}

// GetCategories - return item categories
func (l *Listing) GetCategories() ([]string, error) {
	type categories struct {
		Item struct {
			Categories []string `json:"categories"`
		} `json:"item"`
	}
	var t categories
	err := json.Unmarshal(l.ListingBytes, &t)
	if err != nil {
		return nil, err
	}
	return t.Item.Categories, nil
}

// GetGrams - return item wt in grams
func (l *Listing) GetGrams() (float32, error) {
	type grams struct {
		Item struct {
			Grams float32 `json:"grams"`
		} `json:"item"`
	}
	var t grams
	err := json.Unmarshal(l.ListingBytes, &t)
	if err != nil {
		return 0, err
	}
	return t.Item.Grams, nil
}

// GetCondition - return item condition
func (l *Listing) GetCondition() (string, error) {
	type condition struct {
		Item struct {
			Condition string `json:"condition"`
		} `json:"item"`
	}
	var t condition
	err := json.Unmarshal(l.ListingBytes, &t)
	if err != nil {
		return "", err
	}
	return t.Item.Condition, nil
}

// GetImages - return item images
func (l *Listing) GetImages() ([]*pb.Listing_Item_Image, error) {
	type images struct {
		Item struct {
			Images []struct {
				Filename string `json:"filename"`
				Original string `json:"original"`
				Large    string `json:"large"`
				Medium   string `json:"medium"`
				Small    string `json:"small"`
				Tiny     string `json:"tiny"`
			} `json:"images"`
		} `json:"item"`
	}
	var i images
	err := json.Unmarshal(l.ListingBytes, &i)
	if err != nil {
		return nil, err
	}
	img := []*pb.Listing_Item_Image{}
	for _, elem := range i.Item.Images {
		img0 := pb.Listing_Item_Image{
			Filename: elem.Filename,
			Original: elem.Original,
			Large:    elem.Large,
			Medium:   elem.Medium,
			Small:    elem.Small,
			Tiny:     elem.Tiny,
		}
		img = append(img, &img0)
	}
	return img, nil
}

// GetOptions - return item options
func (l *Listing) GetOptions() ([]*pb.Listing_Item_Option, error) {
	type options struct {
		Item struct {
			Options []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Variants    []struct {
					Name  string `json:"name"`
					Image struct {
						Filename string `json:"filename"`
						Original string `json:"original"`
						Large    string `json:"large"`
						Medium   string `json:"medium"`
						Small    string `json:"small"`
						Tiny     string `json:"tiny"`
					} `json:"image"`
				} `json:"variants"`
			} `json:"options"`
		} `json:"item"`
	}
	var o options
	err := json.Unmarshal(l.ListingBytes, &o)
	if err != nil {
		return nil, err
	}
	opts := []*pb.Listing_Item_Option{}
	for _, elem := range o.Item.Options {
		opt := pb.Listing_Item_Option{
			Name:        elem.Name,
			Description: elem.Description,
		}
		variants := []*pb.Listing_Item_Option_Variant{}
		for _, v := range elem.Variants {
			var0 := pb.Listing_Item_Option_Variant{
				Name: v.Name,
				Image: &pb.Listing_Item_Image{
					Filename: v.Image.Filename,
					Original: v.Image.Original,
					Large:    v.Image.Large,
					Medium:   v.Image.Medium,
					Small:    v.Image.Small,
					Tiny:     v.Image.Tiny,
				},
			}
			variants = append(variants, &var0)
		}
		opt.Variants = variants
		opts = append(opts, &opt)
	}
	return opts, nil
}

// GetSkus - return item skus
func (l *Listing) GetSkus() ([]*pb.Listing_Item_Sku, error) {
	retSkus := []*pb.Listing_Item_Sku{}
	type skus struct {
		Item struct {
			Skus []struct {
				VariantCombo []uint32    `json:"variantcombo"`
				ProductID    string      `json:"productID"`
				Quantity     int64       `json:"quantity"`
				Surcharge    interface{} `json:"surcharge"`
			} `json:"skus"`
		} `json:"item"`
	}
	var s skus
	err := json.Unmarshal(l.ListingBytes, &s)
	if err != nil {
		return nil, err
	}
	for _, elem := range s.Item.Skus {
		sku := pb.Listing_Item_Sku{
			VariantCombo: elem.VariantCombo,
			ProductID:    elem.ProductID,
			Quantity:     elem.Quantity,
		}
		surchargeValue := &pb.CurrencyValue{}
		//var ok bool
		switch l.ListingVersion {
		case 3, 4:
			{
				surcharge, ok := elem.Surcharge.(int64)
				if !ok {
					return nil, errors.New("invalid surcharge value")
				}
				surchargeValue.Amount = big.NewInt(surcharge).String()
				type pricingCurrency struct {
					Metadata struct {
						PricingCurrency string `json:"pricingCurrency"`
					} `json:"metadata"`
				}
				var pc pricingCurrency
				json.Unmarshal(l.ListingBytes, &pc)
				surchargeValue.Currency = &pb.CurrencyDefinition{
					Code:         pc.Metadata.PricingCurrency,
					Divisibility: 8,
				}
			}
		case 5:
			{
				/*
					log.Info("elem.surcharge  : ", elem.Surcharge)
					surchargeMap, ok := elem.Surcharge.(map[string]interface{})
					log.Info("sur map ", surchargeMap, "   ok  ", ok)
					if !ok {
						return nil, errors.New("invalid surcharge value")
					}
					amt0, ok := surchargeMap["amount"]
					log.Info("amt0 ", amt0, "   ok  ", ok)
					if !ok {
						return nil, errors.New("invalid surcharge value")
					}
					amt, ok := amt0.(string)
					log.Info("amt ", amt, "   ok  ", ok)
					if !ok {
						return nil, errors.New("invalid surcharge value")
					}
					surchargeValue.Amount = amt
					curr0, ok := surchargeMap["currency"]
					log.Info("curr0 ", curr0, "   ok  ", ok)
					if !ok {
						return nil, errors.New("invalid surcharge value")
					}
					curr, ok := curr0.(map[string]interface{})
					if !ok {
						return nil, errors.New("invalid surcharge value")
					}
					code0, ok := curr["code"]
					log.Info("code0 ", code0, "   ok  ", ok)
					if !ok {
						return nil, errors.New("invalid surcharge value")
					}
					code, ok := code0.(string)
					log.Info("code  ", code, "   ok  ", ok)
					if !ok {
						return nil, errors.New("invalid surcharge value")
					}
					div0, ok := curr["divisibility"]
					log.Info("div0 ", div0, "   ok  ", ok)
					if !ok {
						return nil, errors.New("invalid surcharge value")
					}
					div, ok := div0.(float64)
					log.Info("div ", div, "   ok  ", ok)
					if !ok {
						return nil, errors.New("invalid surcharge value")
					}
					surchargeValue.Currency = &pb.CurrencyDefinition{
						Code:         code,
						Divisibility: uint32(div),
					}
				*/
				surchargeValue, err = extractCurrencyValue(elem.Surcharge)
				if err != nil {
					return nil, errors.New("invalid surcharge value")
				}
			}
		}
		sku.SurchargeValue = surchargeValue
		retSkus = append(retSkus, &sku)
	}
	return retSkus, nil
}

// GetItem - return item
func (l *Listing) GetItem() (*pb.Listing_Item, error) {
	title, err := l.GetTitle()
	if err != nil {
		return nil, err
	}
	description, err := l.GetDescription()
	if err != nil {
		return nil, err
	}
	processingtime, err := l.GetProcessingTime()
	if err != nil {
		return nil, err
	}
	nsfw, err := l.GetNsfw()
	if err != nil {
		return nil, err
	}
	tags, err := l.GetTags()
	if err != nil {
		return nil, err
	}
	images, err := l.GetImages()
	if err != nil {
		return nil, err
	}
	categories, err := l.GetCategories()
	if err != nil {
		return nil, err
	}
	grams, err := l.GetGrams()
	if err != nil {
		return nil, err
	}
	condition, err := l.GetCondition()
	if err != nil {
		return nil, err
	}
	options, err := l.GetOptions()
	if err != nil {
		return nil, err
	}
	skus, err := l.GetSkus()
	if err != nil {
		return nil, err
	}
	price, err := l.GetPrice()
	if err != nil {
		return nil, err
	}
	i := pb.Listing_Item{
		Title:          title,
		Description:    description,
		ProcessingTime: processingtime,
		Nsfw:           nsfw,
		Tags:           tags,
		Images:         images,
		Categories:     categories,
		Grams:          grams,
		Condition:      condition,
		Options:        options,
		Skus:           skus,
		PriceValue: &pb.CurrencyValue{
			Amount: price.Amount.String(),
			Currency: &pb.CurrencyDefinition{
				Code:         price.Currency.Code.String(),
				Divisibility: uint32(price.Currency.Divisibility),
			},
		},
	}
	return &i, nil
}

// GetExpiry return listing expiry
func (l *Listing) GetExpiry() (*timestamp.Timestamp, error) {
	log.Info("in get expiry ")
	type expiry struct {
		Metadata struct {
			Expiry string `json:"expiry"`
		} `json:"metadata"`
	}
	var exp expiry
	err := json.Unmarshal(l.ListingBytes, &exp)
	log.Info("lets deeee   : ", exp, "   ", err)
	if err != nil {
		return nil, err
	}
	t := new(timestamp.Timestamp)

	t0, err := time.Parse(time.RFC3339Nano, exp.Metadata.Expiry)
	log.Info(" parsed time  :  ", t0, "    err  : ", err)
	if err != nil {
		return nil, err
	}
	//target.Field(0).SetInt(int64(t.Unix()))
	//target.Field(1).SetInt(int64(t.Nanosecond()))

	t.Seconds = t0.Unix()
	t.Nanos = int32(t0.Nanosecond())

	//err = jsonpb.UnmarshalString(exp.Metadata.Expiry, t)
	//if err != nil {
	//	return nil, err
	//}
	log.Info("timestamp   : ", t)
	return t, nil
}

// GetLanguage return listing's language
func (l *Listing) GetLanguage() (string, error) {
	retVal := ""
	type lang struct {
		Metadata struct {
			Language string `json:"language"`
		} `json:"metadata"`
	}
	var ll lang
	err := json.Unmarshal(l.ListingBytes, &ll)
	if err != nil {
		return retVal, err
	}
	return ll.Metadata.Language, nil
}

// GetEscrowTimeout return listing's escrow timeout in hours
func (l *Listing) GetEscrowTimeout() (uint32, error) {
	type escrow struct {
		Metadata struct {
			EscrowTimeout uint32 `json:"escrowTimeoutHours"`
		} `json:"metadata"`
	}
	var e escrow
	err := json.Unmarshal(l.ListingBytes, &e)
	if err != nil {
		return 0, err
	}
	return e.Metadata.EscrowTimeout, nil
}

// GetPriceModifier return listing's price modifier
func (l *Listing) GetPriceModifier() (float32, error) {
	type priceMod struct {
		Metadata struct {
			PriceModifier float32 `json:"priceModifier"`
		} `json:"metadata"`
	}
	var p priceMod
	err := json.Unmarshal(l.ListingBytes, &p)
	if err != nil {
		return 0, err
	}
	return p.Metadata.PriceModifier, nil
}

// GetPricingCurrencyDefn return the listing currency definition
func (l *Listing) GetPricingCurrencyDefn() (*pb.CurrencyDefinition, error) {
	retVal := &pb.CurrencyDefinition{}
	contractType, err := l.GetContractType()
	if err != nil {
		return nil, err
	}
	switch l.ListingVersion {
	case 3, 4:
		{
			if contractType == "CRYPTOCURRENCY" {
				type coinType struct {
					Metadata struct {
						CoinType string `json:"coinType"`
					} `json:"metadata"`
				}
				var c coinType
				err := json.Unmarshal(l.ListingBytes, &c)
				if err != nil {
					return nil, err
				}
				retVal = &pb.CurrencyDefinition{
					Code:         c.Metadata.CoinType,
					Divisibility: 8,
				}
			} else {
				type pricingCurrency struct {
					Metadata struct {
						PricingCurrency string `json:"pricingCurrency"`
					} `json:"metadata"`
				}
				var pc pricingCurrency
				err := json.Unmarshal(l.ListingBytes, &pc)
				if err != nil {
					return nil, err
				}
				retVal = &pb.CurrencyDefinition{
					Code:         pc.Metadata.PricingCurrency,
					Divisibility: 8,
				}
			}
		}
	case 5:
		{
			type currdefn struct {
				Metadata struct {
					PricingCurrencyDefn struct {
						Code         string `json:"code"`
						Divisibility uint   `json:"divisibility"`
					} `json:"pricingCurrency"`
				} `json:"metadata"`
			}
			var p currdefn
			err = json.Unmarshal(l.ListingBytes, &p)
			if err != nil {
				return nil, err
			}
			retVal = &pb.CurrencyDefinition{
				Code:         p.Metadata.PricingCurrencyDefn.Code,
				Divisibility: uint32(p.Metadata.PricingCurrencyDefn.Divisibility),
			}
		}
	}
	return retVal, nil
}

// GetMetadata return metadata
func (l *Listing) GetMetadata() (*pb.Listing_Metadata, error) {
	ct, err := l.GetContractType()
	log.Info("ct   err  : ", ct, err)
	if err != nil {
		return nil, err
	}
	ct0, exists := pb.Listing_Metadata_ContractType_value[ct]
	if !exists {
		return nil, errors.New("invalid metadata contractType")
	}
	frmt, err := l.GetFormat()
	log.Info("frmt   err  : ", frmt, err)
	if err != nil {
		return nil, err
	}
	frmt0, exists := pb.Listing_Metadata_Format_value[frmt]
	if !exists {
		return nil, errors.New("invalid metadata format")
	}
	expiry, err := l.GetExpiry()
	log.Info("expiry   err  : ", expiry, err)
	if err != nil {
		return nil, err
	}
	currs, err := l.GetAcceptedCurrencies()
	log.Info("currs   err  : ", currs, err)
	if err != nil {
		return nil, err
	}
	lang, err := l.GetLanguage()
	log.Info("lang   err  : ", lang, err)
	if err != nil {
		return nil, err
	}
	escrowTimout, err := l.GetEscrowTimeout()
	log.Info("escrw   err  : ", escrowTimout, err)
	if err != nil {
		return nil, err
	}
	priceMod, err := l.GetPriceModifier()
	log.Info("pricemod   err  : ", priceMod, err)
	if err != nil {
		return nil, err
	}
	currDefn, err := l.GetPricingCurrencyDefn()
	log.Info("currdef   err  : ", currDefn, err)
	if err != nil {
		return nil, err
	}
	m := pb.Listing_Metadata{
		Version:             l.ListingVersion,
		ContractType:        pb.Listing_Metadata_ContractType(ct0),
		Format:              pb.Listing_Metadata_Format(frmt0),
		Expiry:              expiry,
		AcceptedCurrencies:  currs,
		Language:            lang,
		EscrowTimeoutHours:  escrowTimout,
		PriceModifier:       priceMod,
		PricingCurrencyDefn: currDefn,
	}
	return &m, nil
}

// GetSOName returns shipping option name

// GetShippingOptions - return shippingOptions
func (l *Listing) GetShippingOptions() ([]*pb.Listing_ShippingOption, error) {
	options := []*pb.Listing_ShippingOption{}
	type shippingOptions struct {
		ShippingOptions []struct {
			Name     string   `json:"name"`
			Type     string   `json:"type"`
			Regions  []string `json:"regions"`
			Services []struct {
				Name              string      `json:"name"`
				EstimatedDelivery string      `json:"estimatedDelivery"`
				Price             interface{} `json:"price"`
				AdditionalPrice   interface{} `json:"addtionalPrice"`
			} `json:"services"`
		} `json:"shippingOptions"`
	}
	var sopts shippingOptions
	err := json.Unmarshal(l.ListingBytes, &sopts)
	if err != nil {
		return nil, err
	}
	for _, elem := range sopts.ShippingOptions {
		sType, ok := pb.Listing_ShippingOption_ShippingType_value[elem.Type]
		if !ok {
			return nil, errors.New("invalid shipping option type specified")
		}
		countryCodes := []pb.CountryCode{}
		for _, c := range elem.Regions {
			cCode, ok := pb.CountryCode_value[c]
			if ok {
				countryCodes = append(countryCodes, pb.CountryCode(cCode))
			}
		}
		services := []*pb.Listing_ShippingOption_Service{}

		for _, svc := range elem.Services {
			priceValue := new(pb.CurrencyValue)
			addnPriceValue := new(pb.CurrencyValue)
			//var ok bool
			switch l.ListingVersion {
			case 3, 4:
				{
					if svc.Price != nil {
						price, ok := svc.Price.(uint64)
						if !ok {
							return nil, errors.New("invalid service price value")
						}
						priceValue.Amount = big.NewInt(int64(price)).String()
					} else {
						priceValue.Amount = big.NewInt(0).String()
					}

					if svc.AdditionalPrice != nil {
						addnPrice, ok := svc.AdditionalPrice.(uint64)
						if !ok {
							return nil, errors.New("invalid service additional price value")
						}
						addnPriceValue.Amount = big.NewInt(int64(addnPrice)).String()
					} else {
						addnPriceValue.Amount = big.NewInt(0).String()
					}

					type pricingCurrency struct {
						Metadata struct {
							PricingCurrency string `json:"pricingCurrency"`
						} `json:"metadata"`
					}
					var pc pricingCurrency
					err = json.Unmarshal(l.ListingBytes, &pc)
					if err != nil {
						return nil, err
					}
					priceValue.Currency = &pb.CurrencyDefinition{
						Code:         pc.Metadata.PricingCurrency,
						Divisibility: 8,
					}
					addnPriceValue.Currency = &pb.CurrencyDefinition{
						Code:         pc.Metadata.PricingCurrency,
						Divisibility: 8,
					}
				}
			case 5:
				{
					priceValue, err = extractCurrencyValue(svc.Price) //.(pb.CurrencyValue)
					if err != nil {
						return nil, errors.New("invalid price value")
					}
					addnPriceValue, err = extractCurrencyValue(svc.AdditionalPrice) //.(pb.CurrencyValue)
					if err != nil {
						return nil, errors.New("invalid price value")
					}
				}
			}
			srv := pb.Listing_ShippingOption_Service{
				Name:                     svc.Name,
				EstimatedDelivery:        svc.EstimatedDelivery,
				PriceValue:               priceValue,
				AdditionalItemPriceValue: addnPriceValue,
			}
			services = append(services, &srv)
		}
		shipOption := pb.Listing_ShippingOption{
			Name:     elem.Name,
			Type:     pb.Listing_ShippingOption_ShippingType(sType),
			Regions:  countryCodes,
			Services: services,
		}
		options = append(options, &shipOption)
	}
	return options, nil
}

func extractCurrencyValue(v interface{}) (*pb.CurrencyValue, error) {
	value := new(pb.CurrencyValue)
	if v == nil {
		return value, nil
	}
	log.Info("v int  : ", v)
	vMap, ok := v.(map[string]interface{})
	log.Info("v map ", vMap, "   ok  ", ok)
	if !ok {
		return nil, errors.New("invalid currency value")
	}
	amt0, ok := vMap["amount"]
	log.Info("amt0 ", amt0, "   ok  ", ok)
	if !ok {
		return nil, errors.New("invalid currency value")
	}
	amt, ok := amt0.(string)
	log.Info("amt ", amt, "   ok  ", ok)
	if !ok {
		return nil, errors.New("invalid currency value")
	}
	value.Amount = amt
	curr0, ok := vMap["currency"]
	log.Info("curr0 ", curr0, "   ok  ", ok)
	if !ok {
		return nil, errors.New("invalid currency value")
	}
	curr, ok := curr0.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid currency value")
	}
	code0, ok := curr["code"]
	log.Info("code0 ", code0, "   ok  ", ok)
	if !ok {
		return nil, errors.New("invalid currency value")
	}
	code, ok := code0.(string)
	log.Info("code  ", code, "   ok  ", ok)
	if !ok {
		return nil, errors.New("invalid currency value")
	}
	div0, ok := curr["divisibility"]
	log.Info("div0 ", div0, "   ok  ", ok)
	if !ok {
		return nil, errors.New("invalid currency value")
	}
	div, ok := div0.(float64)
	log.Info("div ", div, "   ok  ", ok)
	if !ok {
		return nil, errors.New("invalid currency value")
	}
	value.Currency = &pb.CurrencyDefinition{
		Code:         code,
		Divisibility: uint32(div),
	}

	return value, nil
}

// GetTaxes - return taxes
func (l *Listing) GetTaxes() ([]*pb.Listing_Tax, error) {
	ret := []*pb.Listing_Tax{}
	type taxes struct {
		Taxes []struct {
			Type       string   `json:"taxtype"`
			Regions    []string `json:"taxRegions"`
			Shipping   bool     `json:"taxShipping"`
			Percentage float32  `json:"Percentage"`
		} `json:"taxes"`
	}
	var t taxes
	err := json.Unmarshal(l.ListingBytes, &t)
	if err != nil {
		return nil, err
	}
	for _, elem := range t.Taxes {
		countryCodes := []pb.CountryCode{}
		for _, c := range elem.Regions {
			cCode, ok := pb.CountryCode_value[c]
			if ok {
				countryCodes = append(countryCodes, pb.CountryCode(cCode))
			}
		}
		tax := pb.Listing_Tax{
			TaxType:     elem.Type,
			TaxRegions:  countryCodes,
			TaxShipping: elem.Shipping,
			Percentage:  elem.Percentage,
		}
		ret = append(ret, &tax)
	}
	return ret, nil
}

// GetCoupons - return coupons
func (l *Listing) GetCoupons() ([]*pb.Listing_Coupon, error) {
	ret := []*pb.Listing_Coupon{}
	type coupons struct {
		Coupons []interface{} `json:"coupons"`
	}
	var c coupons
	err := json.Unmarshal(l.ListingBytes, &c)
	if err != nil {
		return nil, err
	}
	for _, elem := range c.Coupons {
		ret1 := new(pb.Listing_Coupon)
		b, err := json.Marshal(elem)
		if err != nil {
			return nil, err
		}
		err = jsonpb.UnmarshalString(string(b), ret1)
		if err != nil {
			return nil, err
		}
		ret = append(ret, ret1)
	}
	return ret, nil
}

/*
	type Listing struct {
		Slug                 string                    `protobuf:"bytes,1,opt,name=slug,proto3" json:"slug,omitempty"`
		VendorID             *ID                       `protobuf:"bytes,2,opt,name=vendorID,proto3" json:"vendorID,omitempty"`
		Metadata             *Listing_Metadata         `protobuf:"bytes,3,opt,name=metadata,proto3" json:"metadata,omitempty"`
		Item                 *Listing_Item             `protobuf:"bytes,4,opt,name=item,proto3" json:"item,omitempty"`
		ShippingOptions      []*Listing_ShippingOption `protobuf:"bytes,5,rep,name=shippingOptions,proto3" json:"shippingOptions,omitempty"`
		Taxes                []*Listing_Tax            `protobuf:"bytes,6,rep,name=taxes,proto3" json:"taxes,omitempty"`
		Coupons              []*Listing_Coupon         `protobuf:"bytes,7,rep,name=coupons,proto3" json:"coupons,omitempty"`
		Moderators           []string                  `protobuf:"bytes,8,rep,name=moderators,proto3" json:"moderators,omitempty"`
		TermsAndConditions   string                    `protobuf:"bytes,9,opt,name=termsAndConditions,proto3" json:"termsAndConditions,omitempty"`
		RefundPolicy         string                    `protobuf:"bytes,10,opt,name=refundPolicy,proto3" json:"refundPolicy,omitempty"`
	}
*/

// GetProtoListing - return pb.Listing
func (l *Listing) GetProtoListing() (*pb.Listing, error) {
	log.Info("in get proto listing ... ")
	//log.Info(l.ListingBytes)

	//log.Info("proto listing : ")
	//log.Info(l.ProtoListing)
	if l.ProtoListing != nil {
		return l.ProtoListing, nil
	}

	slug, err := l.GetSlug()
	log.Info("slug   err  : ", slug, err)
	if err != nil {
		return nil, err
	}

	vendor, err := l.GetVendorID()
	log.Info("vendor   err  : ", vendor, err)
	if err != nil {
		return nil, err
	}

	metadata, err := l.GetMetadata()
	log.Info("meta   err  : ", metadata, err)
	if err != nil {
		return nil, err
	}

	item, err := l.GetItem()
	log.Info("item   err  : ", item, err)
	if err != nil {
		return nil, err
	}

	shippingOptions, err := l.GetShippingOptions()
	log.Info("shipopt   err  : ", shippingOptions, err)
	if err != nil {
		return nil, err
	}

	taxes, err := l.GetTaxes()
	log.Info("taxes   err  : ", taxes, err)
	if err != nil {
		return nil, err
	}

	coupons, err := l.GetCoupons()
	log.Info("coupons   err  : ", coupons, err)
	if err != nil {
		return nil, err
	}

	mods, err := l.GetModerators()
	log.Info("mods   err  : ", mods, err)
	if err != nil {
		return nil, err
	}

	tnc, err := l.GetTermsAndConditions()
	log.Info("tnc   err  : ", tnc, err)
	if err != nil {
		return nil, err
	}

	rpol, err := l.GetRefundPolicy()
	if err != nil {
		return nil, err
	}

	pbl := pb.Listing{
		Slug:               slug,
		VendorID:           vendor,
		Metadata:           metadata,
		Item:               item,
		ShippingOptions:    shippingOptions,
		Taxes:              taxes,
		Coupons:            coupons,
		Moderators:         mods,
		TermsAndConditions: tnc,
		RefundPolicy:       rpol,
	}
	l.ProtoListing = &pbl
	return &pbl, nil
}

// Sign - return signedListing
func (l *Listing) Sign(n *core.IpfsNode, timeout, expectedDivisibility uint32,
	handle string, key *hdkeychain.ExtendedKey, dStore *Datastore) (SignedListing, error) {
	listing, err := l.GetProtoListing()
	if err != nil {
		return SignedListing{}, err
	}
	// Set inventory to the default as it's not part of the contract
	for _, s := range listing.Item.Skus {
		s.Quantity = 0
	}

	sl := new(pb.SignedListing)

	rsl := SignedListing{
		ProtoSignedListing: sl,
	}

	// Validate accepted currencies
	if len(listing.Metadata.AcceptedCurrencies) == 0 {
		return rsl, errors.New("accepted currencies must be set")
	}
	if listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY && len(listing.Metadata.AcceptedCurrencies) != 1 {
		return rsl, errors.New("a cryptocurrency listing must only have one accepted currency")
	}

	// Sanitize a few critical fields
	if listing.Item == nil {
		return rsl, errors.New("no item in listing")
	}
	sanitizer := bluemonday.UGCPolicy()
	for _, opt := range listing.Item.Options {
		opt.Name = sanitizer.Sanitize(opt.Name)
		for _, v := range opt.Variants {
			v.Name = sanitizer.Sanitize(v.Name)
		}
	}
	for _, so := range listing.ShippingOptions {
		so.Name = sanitizer.Sanitize(so.Name)
		for _, serv := range so.Services {
			serv.Name = sanitizer.Sanitize(serv.Name)
		}
	}

	// Check the listing data is correct for continuing
	testingEnabled := timeout > 1
	if err := ValidateListing(l, testingEnabled, expectedDivisibility); err != nil {
		return rsl, err
	}

	// Set listing version
	listing.Metadata.Version = ListingVersion

	// Add the vendor ID to the listing
	id := new(pb.ID)
	id.PeerID = n.Identity.Pretty()
	pubkey, err := n.PrivateKey.GetPublic().Bytes()
	if err != nil {
		return rsl, err
	}

	p := new(pb.ID_Pubkeys)
	p.Identity = pubkey
	ecPubKey, err := key.ECPubKey()
	if err != nil {
		return rsl, err
	}
	p.Bitcoin = ecPubKey.SerializeCompressed()
	id.Pubkeys = p
	listing.VendorID = id

	// Sign the GUID with the Bitcoin key
	ecPrivKey, err := key.ECPrivKey()
	if err != nil {
		return rsl, err
	}
	sig, err := ecPrivKey.Sign([]byte(id.PeerID))
	if err != nil {
		return rsl, err
	}
	id.BitcoinSig = sig.Serialize()

	// Update coupon db
	(*dStore).Coupons().Delete(listing.Slug)
	var couponsToStore []Coupon
	for i, coupon := range listing.Coupons {
		hash := coupon.GetHash()
		code := coupon.GetDiscountCode()
		_, err := mh.FromB58String(hash)
		if err != nil {
			couponMH, err := ipfs.EncodeMultihash([]byte(code))
			if err != nil {
				return rsl, err
			}

			listing.Coupons[i].Code = &pb.Listing_Coupon_Hash{Hash: couponMH.B58String()}
			hash = couponMH.B58String()
		}
		c := Coupon{Slug: listing.Slug, Code: code, Hash: hash}
		couponsToStore = append(couponsToStore, c)
	}
	err = (*dStore).Coupons().Put(couponsToStore)
	if err != nil {
		return rsl, err
	}

	// Sign listing
	serializedListing, err := proto.Marshal(listing)
	if err != nil {
		return rsl, err
	}
	idSig, err := n.PrivateKey.Sign(serializedListing)
	if err != nil {
		return rsl, err
	}

	sl.Listing = listing
	sl.Signature = idSig
	rsl.ProtoSignedListing = sl
	rsl.Listing = *l
	return rsl, nil
}

// ValidateCryptoListing - check cryptolisting
func (l *Listing) ValidateCryptoListing(div uint32) error {
	listing, err := l.GetProtoListing()
	if err != nil {
		return err
	}
	return validateCryptocurrencyListing(listing, div)
}

// ValidateSkus - check listing skus
func (l *Listing) ValidateSkus() error {
	listing, err := l.GetProtoListing()
	if err != nil {
		return err
	}
	return validateListingSkus(listing)
}

// GetInventory - returns a map of skus and quantityies
func (l *Listing) GetInventory() (map[int]int64, error) {
	listing, err := l.GetProtoListing()
	if err != nil {
		return nil, err
	}
	inventory := make(map[int]int64)
	for i, s := range listing.Item.Skus {
		inventory[i] = s.Quantity
	}
	return inventory, nil
}

/* Performs a ton of checks to make sure the listing is formatted correctly. We should not allow
   invalid listings to be saved or purchased as it can lead to ambiguity when moderating a dispute
   or possible attacks. This function needs to be maintained in conjunction with contracts.proto */
func ValidateListing(l *Listing, testnet bool, expectedDivisibility uint32) (err error) {
	listing, err := l.GetProtoListing()
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case string:
				err = errors.New(x)
			case error:
				err = x
			default:
				err = errors.New("unknown panic")
			}
		}
	}()

	// Slug
	if listing.Slug == "" {
		return errors.New("slug must not be empty")
	}
	if len(listing.Slug) > SentenceMaxCharacters {
		return fmt.Errorf("slug is longer than the max of %d", SentenceMaxCharacters)
	}
	if strings.Contains(listing.Slug, " ") {
		return errors.New("slugs cannot contain spaces")
	}
	if strings.Contains(listing.Slug, "/") {
		return errors.New("slugs cannot contain file separators")
	}

	// Metadata
	if listing.Metadata == nil {
		return errors.New("missing required field: Metadata")
	}
	if listing.Metadata.ContractType > pb.Listing_Metadata_CRYPTOCURRENCY {
		return errors.New("invalid contract type")
	}
	if listing.Metadata.Format > pb.Listing_Metadata_MARKET_PRICE {
		return errors.New("invalid listing format")
	}
	if listing.Metadata.Expiry == nil {
		return errors.New("missing required field: Expiry")
	}
	if time.Unix(listing.Metadata.Expiry.Seconds, 0).Before(time.Now()) {
		return errors.New("listing expiration must be in the future")
	}
	if len(listing.Metadata.Language) > WordMaxCharacters {
		return fmt.Errorf("language is longer than the max of %d characters", WordMaxCharacters)
	}

	if !testnet && listing.Metadata.EscrowTimeoutHours != EscrowTimeout {
		return fmt.Errorf("escrow timeout must be %d hours", EscrowTimeout)
	}
	if len(listing.Metadata.AcceptedCurrencies) == 0 {
		return errors.New("at least one accepted currency must be provided")
	}
	if len(listing.Metadata.AcceptedCurrencies) > MaxListItems {
		return fmt.Errorf("acceptedCurrencies is longer than the max of %d currencies", MaxListItems)
	}
	for _, c := range listing.Metadata.AcceptedCurrencies {
		if len(c) > WordMaxCharacters {
			return fmt.Errorf("accepted currency is longer than the max of %d characters", WordMaxCharacters)
		}
	}

	// Item
	if listing.Item.Title == "" {
		return errors.New("listing must have a title")
	}
	if listing.Metadata.ContractType != pb.Listing_Metadata_CRYPTOCURRENCY && listing.Item.PriceValue.Amount == "0" {
		return errors.New("zero price listings are not allowed")
	}
	if len(listing.Item.Title) > TitleMaxCharacters {
		return fmt.Errorf("title is longer than the max of %d characters", TitleMaxCharacters)
	}
	if len(listing.Item.Description) > DescriptionMaxCharacters {
		return fmt.Errorf("description is longer than the max of %d characters", DescriptionMaxCharacters)
	}
	if len(listing.Item.ProcessingTime) > SentenceMaxCharacters {
		return fmt.Errorf("processing time length must be less than the max of %d", SentenceMaxCharacters)
	}
	if len(listing.Item.Tags) > MaxTags {
		return fmt.Errorf("number of tags exceeds the max of %d", MaxTags)
	}
	for _, tag := range listing.Item.Tags {
		if tag == "" {
			return errors.New("tags must not be empty")
		}
		if len(tag) > WordMaxCharacters {
			return fmt.Errorf("tags must be less than max of %d", WordMaxCharacters)
		}
	}
	if len(listing.Item.Images) == 0 {
		return errors.New("listing must contain at least one image")
	}
	if len(listing.Item.Images) > MaxListItems {
		return fmt.Errorf("number of listing images is greater than the max of %d", MaxListItems)
	}
	for _, img := range listing.Item.Images {
		_, err := cid.Decode(img.Tiny)
		if err != nil {
			return errors.New("tiny image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Small)
		if err != nil {
			return errors.New("small image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Medium)
		if err != nil {
			return errors.New("medium image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Large)
		if err != nil {
			return errors.New("large image hashes must be properly formatted CID")
		}
		_, err = cid.Decode(img.Original)
		if err != nil {
			return errors.New("original image hashes must be properly formatted CID")
		}
		if img.Filename == "" {
			return errors.New("image file names must not be nil")
		}
		if len(img.Filename) > FilenameMaxCharacters {
			return fmt.Errorf("image filename length must be less than the max of %d", FilenameMaxCharacters)
		}
	}
	if len(listing.Item.Categories) > MaxCategories {
		return fmt.Errorf("number of categories must be less than max of %d", MaxCategories)
	}
	for _, category := range listing.Item.Categories {
		if category == "" {
			return errors.New("categories must not be nil")
		}
		if len(category) > WordMaxCharacters {
			return fmt.Errorf("category length must be less than the max of %d", WordMaxCharacters)
		}
	}

	maxCombos := 1
	variantSizeMap := make(map[int]int)
	optionMap := make(map[string]struct{})
	for i, option := range listing.Item.Options {
		if _, ok := optionMap[option.Name]; ok {
			return errors.New("option names must be unique")
		}
		if option.Name == "" {
			return errors.New("options titles must not be empty")
		}
		if len(option.Variants) < 2 {
			return errors.New("options must have more than one variants")
		}
		if len(option.Name) > WordMaxCharacters {
			return fmt.Errorf("option title length must be less than the max of %d", WordMaxCharacters)
		}
		if len(option.Description) > SentenceMaxCharacters {
			return fmt.Errorf("option description length must be less than the max of %d", SentenceMaxCharacters)
		}
		if len(option.Variants) > MaxListItems {
			return fmt.Errorf("number of variants is greater than the max of %d", MaxListItems)
		}
		varMap := make(map[string]struct{})
		for _, variant := range option.Variants {
			if _, ok := varMap[variant.Name]; ok {
				return errors.New("variant names must be unique")
			}
			if len(variant.Name) > WordMaxCharacters {
				return fmt.Errorf("variant name length must be less than the max of %d", WordMaxCharacters)
			}
			if variant.Image != nil && (variant.Image.Filename != "" ||
				variant.Image.Large != "" || variant.Image.Medium != "" || variant.Image.Small != "" ||
				variant.Image.Tiny != "" || variant.Image.Original != "") {
				_, err := cid.Decode(variant.Image.Tiny)
				if err != nil {
					return errors.New("tiny image hashes must be properly formatted CID")
				}
				_, err = cid.Decode(variant.Image.Small)
				if err != nil {
					return errors.New("small image hashes must be properly formatted CID")
				}
				_, err = cid.Decode(variant.Image.Medium)
				if err != nil {
					return errors.New("medium image hashes must be properly formatted CID")
				}
				_, err = cid.Decode(variant.Image.Large)
				if err != nil {
					return errors.New("large image hashes must be properly formatted CID")
				}
				_, err = cid.Decode(variant.Image.Original)
				if err != nil {
					return errors.New("original image hashes must be properly formatted CID")
				}
				if variant.Image.Filename == "" {
					return errors.New("image file names must not be nil")
				}
				if len(variant.Image.Filename) > FilenameMaxCharacters {
					return fmt.Errorf("image filename length must be less than the max of %d", FilenameMaxCharacters)
				}
			}
			varMap[variant.Name] = struct{}{}
		}
		variantSizeMap[i] = len(option.Variants)
		maxCombos *= len(option.Variants)
		optionMap[option.Name] = struct{}{}
	}

	if len(listing.Item.Skus) > maxCombos {
		return errors.New("more skus than variant combinations")
	}
	comboMap := make(map[string]bool)
	for _, sku := range listing.Item.Skus {
		if maxCombos > 1 && len(sku.VariantCombo) == 0 {
			return errors.New("skus must specify a variant combo when options are used")
		}
		if len(sku.ProductID) > WordMaxCharacters {
			return fmt.Errorf("product ID length must be less than the max of %d", WordMaxCharacters)
		}
		formatted, err := json.Marshal(sku.VariantCombo)
		if err != nil {
			return err
		}
		_, ok := comboMap[string(formatted)]
		if !ok {
			comboMap[string(formatted)] = true
		} else {
			return errors.New("duplicate sku")
		}
		if len(sku.VariantCombo) != len(listing.Item.Options) {
			return errors.New("incorrect number of variants in sku combination")
		}
		for i, combo := range sku.VariantCombo {
			if int(combo) > variantSizeMap[i] {
				return errors.New("invalid sku variant combination")
			}
		}

	}

	// Taxes
	if len(listing.Taxes) > MaxListItems {
		return fmt.Errorf("number of taxes is greater than the max of %d", MaxListItems)
	}
	for _, tax := range listing.Taxes {
		if tax.TaxType == "" {
			return errors.New("tax type must be specified")
		}
		if len(tax.TaxType) > WordMaxCharacters {
			return fmt.Errorf("tax type length must be less than the max of %d", WordMaxCharacters)
		}
		if len(tax.TaxRegions) == 0 {
			return errors.New("tax must specify at least one region")
		}
		if len(tax.TaxRegions) > MaxCountryCodes {
			return fmt.Errorf("number of tax regions is greater than the max of %d", MaxCountryCodes)
		}
		if tax.Percentage == 0 || tax.Percentage > 100 {
			return errors.New("tax percentage must be between 0 and 100")
		}
	}

	// Coupons
	if len(listing.Coupons) > MaxListItems {
		return fmt.Errorf("number of coupons is greater than the max of %d", MaxListItems)
	}
	for _, coupon := range listing.Coupons {
		if len(coupon.Title) > CouponTitleMaxCharacters {
			return fmt.Errorf("coupon title length must be less than the max of %d", SentenceMaxCharacters)
		}
		if len(coupon.GetDiscountCode()) > CodeMaxCharacters {
			return fmt.Errorf("coupon code length must be less than the max of %d", CodeMaxCharacters)
		}
		if coupon.GetPercentDiscount() > 100 {
			return errors.New("percent discount cannot be over 100 percent")
		}
		n, _ := new(big.Int).SetString(listing.Item.PriceValue.Amount, 10)
		discountVal := coupon.GetPriceDiscountValue()
		flag := false
		if discountVal != nil {
			discount0, _ := new(big.Int).SetString(discountVal.Amount, 10)
			if n.Cmp(discount0) < 0 {
				return errors.New("price discount cannot be greater than the item price")
			}
			if n.Cmp(discount0) == 0 {
				flag = true
			}
		}
		if coupon.GetPercentDiscount() == 0 && flag {
			return errors.New("coupons must have at least one positive discount value")
		}
	}

	// Moderators
	if len(listing.Moderators) > MaxListItems {
		return fmt.Errorf("number of moderators is greater than the max of %d", MaxListItems)
	}
	for _, moderator := range listing.Moderators {
		_, err := mh.FromB58String(moderator)
		if err != nil {
			return errors.New("moderator IDs must be multihashes")
		}
	}

	// TermsAndConditions
	if len(listing.TermsAndConditions) > PolicyMaxCharacters {
		return fmt.Errorf("terms and conditions length must be less than the max of %d", PolicyMaxCharacters)
	}

	// RefundPolicy
	if len(listing.RefundPolicy) > PolicyMaxCharacters {
		return fmt.Errorf("refund policy length must be less than the max of %d", PolicyMaxCharacters)
	}

	// Type-specific validations
	if listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD {
		err := validatePhysicalListing(listing)
		if err != nil {
			return err
		}
	} else if listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY {
		err := validateCryptocurrencyListing(listing, expectedDivisibility)
		if err != nil {
			return err
		}
	}

	// Format-specific validations
	if listing.Metadata.Format == pb.Listing_Metadata_MARKET_PRICE {
		err := validateMarketPriceListing(listing)
		if err != nil {
			return err
		}
	}

	return nil
}

func validatePhysicalListing(listing *pb.Listing) error {
	if listing.Metadata.PricingCurrencyDefn.Code == "" {
		return errors.New("listing pricing currency code must not be empty")
	}
	if len(listing.Metadata.PricingCurrencyDefn.Code) > WordMaxCharacters {
		return fmt.Errorf("pricingCurrency is longer than the max of %d characters", WordMaxCharacters)
	}
	if len(listing.Item.Condition) > SentenceMaxCharacters {
		return fmt.Errorf("'Condition' length must be less than the max of %d", SentenceMaxCharacters)
	}
	if len(listing.Item.Options) > MaxListItems {
		return fmt.Errorf("number of options is greater than the max of %d", MaxListItems)
	}

	// ShippingOptions
	if len(listing.ShippingOptions) == 0 {
		return errors.New("must be at least one shipping option for a physical good")
	}
	if len(listing.ShippingOptions) > MaxListItems {
		return fmt.Errorf("number of shipping options is greater than the max of %d", MaxListItems)
	}
	var shippingTitles []string
	for _, shippingOption := range listing.ShippingOptions {
		if shippingOption.Name == "" {
			return errors.New("shipping option title name must not be empty")
		}
		if len(shippingOption.Name) > WordMaxCharacters {
			return fmt.Errorf("shipping option service length must be less than the max of %d", WordMaxCharacters)
		}
		for _, t := range shippingTitles {
			if t == shippingOption.Name {
				return errors.New("shipping option titles must be unique")
			}
		}
		shippingTitles = append(shippingTitles, shippingOption.Name)
		if shippingOption.Type > pb.Listing_ShippingOption_FIXED_PRICE {
			return errors.New("unknown shipping option type")
		}
		if len(shippingOption.Regions) == 0 {
			return errors.New("shipping options must specify at least one region")
		}
		if err := ValidShippingRegion(shippingOption); err != nil {
			return fmt.Errorf("invalid shipping option (%s): %s", shippingOption.String(), err.Error())
		}
		if len(shippingOption.Regions) > MaxCountryCodes {
			return fmt.Errorf("number of shipping regions is greater than the max of %d", MaxCountryCodes)
		}
		if len(shippingOption.Services) == 0 && shippingOption.Type != pb.Listing_ShippingOption_LOCAL_PICKUP {
			return errors.New("at least one service must be specified for a shipping option when not local pickup")
		}
		if len(shippingOption.Services) > MaxListItems {
			return fmt.Errorf("number of shipping services is greater than the max of %d", MaxListItems)
		}
		var serviceTitles []string
		for _, option := range shippingOption.Services {
			if option.Name == "" {
				return errors.New("shipping option service name must not be empty")
			}
			if len(option.Name) > WordMaxCharacters {
				return fmt.Errorf("shipping option service length must be less than the max of %d", WordMaxCharacters)
			}
			for _, t := range serviceTitles {
				if t == option.Name {
					return errors.New("shipping option services names must be unique")
				}
			}
			serviceTitles = append(serviceTitles, option.Name)
			if option.EstimatedDelivery == "" {
				return errors.New("shipping option estimated delivery must not be empty")
			}
			if len(option.EstimatedDelivery) > SentenceMaxCharacters {
				return fmt.Errorf("shipping option estimated delivery length must be less than the max of %d", SentenceMaxCharacters)
			}
		}
	}

	return nil
}

func validateCryptocurrencyListing(listing *pb.Listing, expectedDivisibility uint32) error {
	switch {
	case len(listing.Coupons) > 0:
		return ErrCryptocurrencyListingIllegalField("coupons")
	case len(listing.Item.Options) > 0:
		return ErrCryptocurrencyListingIllegalField("item.options")
	case len(listing.ShippingOptions) > 0:
		return ErrCryptocurrencyListingIllegalField("shippingOptions")
	case len(listing.Item.Condition) > 0:
		return ErrCryptocurrencyListingIllegalField("item.condition")
		//case len(listing.Metadata.PricingCurrency.Code) > 0:
		//	return ErrCryptocurrencyListingIllegalField("metadata.pricingCurrency")
		//case listing.Metadata.CoinType == "":
		//	return ErrCryptocurrencyListingCoinTypeRequired
	}

	if listing.Metadata.PricingCurrencyDefn.Divisibility != expectedDivisibility {
		log.Info("listing.Metadata.PricingCurrency.Divisibility : ", listing.Metadata.PricingCurrencyDefn.Divisibility, "  ", expectedDivisibility)
		return ErrListingCoinDivisibilityIncorrect
	}

	return nil
}

func (l *Listing) SetCryptocurrencyListingDefaults() error {
	listing, err := l.GetProtoListing()
	if err != nil {
		return err
	}
	listing.Coupons = []*pb.Listing_Coupon{}
	listing.Item.Options = []*pb.Listing_Item_Option{}
	listing.ShippingOptions = []*pb.Listing_ShippingOption{}
	listing.Metadata.Format = pb.Listing_Metadata_MARKET_PRICE
	// TODO : set the bytes
	return nil
}

func validateMarketPriceListing(listing *pb.Listing) error {
	n, _ := new(big.Int).SetString(listing.Item.PriceValue.Amount, 10)
	if n.Cmp(big.NewInt(0)) > 0 {
		return ErrMarketPriceListingIllegalField("item.price")
	}

	if listing.Metadata.PriceModifier != 0 {
		listing.Metadata.PriceModifier = float32(int(listing.Metadata.PriceModifier*100.0)) / 100.0
	}

	if listing.Metadata.PriceModifier < PriceModifierMin ||
		listing.Metadata.PriceModifier > PriceModifierMax {
		return ErrPriceModifierOutOfRange{
			Min: PriceModifierMin,
			Max: PriceModifierMax,
		}
	}

	return nil
}

func validateListingSkus(listing *pb.Listing) error {
	if listing.Metadata.ContractType == pb.Listing_Metadata_CRYPTOCURRENCY {
		for _, sku := range listing.Item.Skus {
			if sku.Quantity < 1 {
				return ErrCryptocurrencySkuQuantityInvalid
			}
		}
	}
	return nil
}

func ValidShippingRegion(shippingOption *pb.Listing_ShippingOption) error {
	for _, region := range shippingOption.Regions {
		if int32(region) == 0 {
			return ErrShippingRegionMustBeSet
		}
		_, ok := proto.EnumValueMap("CountryCode")[region.String()]
		if !ok {
			return ErrShippingRegionUndefined
		}
		if ok {
			if int32(region) > 500 {
				return ErrShippingRegionMustNotBeContinent
			}
		}
	}
	return nil
}

func ValidateListingOptions(listingItemOptions []*pb.Listing_Item_Option, itemOptions []option) ([]*pb.Order_Item_Option, error) {
	var validatedListingOptions []*pb.Order_Item_Option
	listingOptions := make(map[string]*pb.Listing_Item_Option)
	for _, opt := range listingItemOptions {
		listingOptions[strings.ToLower(opt.Name)] = opt
	}
	for _, uopt := range itemOptions {
		_, ok := listingOptions[strings.ToLower(uopt.Name)]
		if !ok {
			return nil, errors.New("selected variant not in listing")
		}
		delete(listingOptions, strings.ToLower(uopt.Name))
	}
	if len(listingOptions) > 0 {
		return nil, errors.New("Not all options were selected")
	}

	for _, option := range itemOptions {
		o := &pb.Order_Item_Option{
			Name:  option.Name,
			Value: option.Value,
		}
		validatedListingOptions = append(validatedListingOptions, o)
	}
	return validatedListingOptions, nil
}
