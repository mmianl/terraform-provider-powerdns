package powerdns

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// cryptokeyKeyTypes lists the key roles accepted by the PowerDNS API.
var cryptokeyKeyTypes = []string{"ksk", "zsk", "csk"}

// cryptokeyAlgorithmAliases maps the short mnemonics accepted on create to the
// canonical names PowerDNS reports back, so that writing "ecdsa256" does not
// look like a change once the server answers "ECDSAP256SHA256".
var cryptokeyAlgorithmAliases = map[string]string{
	"rsasha1":       "RSASHA1",
	"rsasha256":     "RSASHA256",
	"rsasha512":     "RSASHA512",
	"ecdsa256":      "ECDSAP256SHA256",
	"ecdsa384":      "ECDSAP384SHA384",
	"ed25519":       "ED25519",
	"ed448":         "ED448",
	"ecc-gost":      "ECC-GOST",
	"rsasha1-nsec3": "RSASHA1-NSEC3-SHA1",
}

// NormalizeCryptokeyAlgorithm resolves an algorithm mnemonic to the canonical
// name used by PowerDNS. Unknown values are upper-cased and returned as-is.
func NormalizeCryptokeyAlgorithm(algorithm string) string {
	if canonical, ok := cryptokeyAlgorithmAliases[strings.ToLower(algorithm)]; ok {
		return canonical
	}
	return strings.ToUpper(algorithm)
}

func resourcePDNSZoneCryptokey() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePDNSZoneCryptokeyCreate,
		ReadContext:   resourcePDNSZoneCryptokeyRead,
		UpdateContext: resourcePDNSZoneCryptokeyUpdate,
		DeleteContext: resourcePDNSZoneCryptokeyDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourcePDNSZoneCryptokeyImport,
		},

		Schema: map[string]*schema.Schema{
			"zone": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: ValidateZoneName,
				Description:  "The zone this DNSSEC key belongs to.",
			},

			"keytype": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice(cryptokeyKeyTypes, false),
				// Some backends store a single combined key and report every key
				// as "csk" regardless of what was requested, which would otherwise
				// show up as a permanent diff.
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return strings.EqualFold(old, new) || strings.EqualFold(old, "csk")
				},
				Description: "The role of the key: ksk, zsk or csk.",
			},

			"algorithm": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
				// PowerDNS echoes back the canonical name (e.g. "ecdsa256" becomes
				// "ECDSAP256SHA256"), so compare the resolved forms and keep the
				// server's spelling in state.
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return NormalizeCryptokeyAlgorithm(old) == NormalizeCryptokeyAlgorithm(new)
				},
				Description: "The signing algorithm, e.g. ecdsa256 or rsasha256. Changing this forces a new key.",
			},

			"bits": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
				ForceNew: true,
				// Algorithms with a fixed key size (the ECDSA and Ed curves) have
				// their size chosen by the server, so an unset value in the
				// configuration must not look like a change.
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return new == "" || new == "0"
				},
				Description: "The key size in bits. Required for the RSA algorithms, " +
					"derived by the server otherwise. Changing this forces a new key.",
			},

			"content": {
				Type:      schema.TypeString,
				Optional:  true,
				ForceNew:  true,
				Sensitive: true,
				Description: "An existing private key in ISC format to import. When omitted, " +
					"PowerDNS generates the key material.",
			},

			"active": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the key is used for signing.",
			},

			"published": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the DNSKEY record is published in the zone.",
			},

			"key_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The numeric ID PowerDNS assigned to this key.",
			},

			"dnskey": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The DNSKEY record for this key.",
			},

			"ds": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The DS records for this key.",
			},

			"cds": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The DS records for this key, filtered by CDS publication settings.",
			},
		},
	}
}

// cryptokeyID combines the zone and the numeric key ID into a Terraform ID.
func cryptokeyID(zone string, id int) string {
	return zone + ":" + strconv.Itoa(id)
}

// parseCryptokeyID splits a Terraform ID back into its zone and numeric key ID.
func parseCryptokeyID(resourceID string) (string, int, error) {
	zone, rawID, found := strings.Cut(resourceID, ":")
	if !found || zone == "" || rawID == "" {
		return "", 0, fmt.Errorf("unknown cryptokey ID format %q, expected zone:key_id", resourceID)
	}

	id, err := strconv.Atoi(rawID)
	if err != nil {
		return "", 0, fmt.Errorf("invalid cryptokey ID %q: %w", resourceID, err)
	}

	return zone, id, nil
}

func resourcePDNSZoneCryptokeyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone := d.Get("zone").(string)
	cryptokey := Cryptokey{
		KeyType:    d.Get("keytype").(string),
		Active:     d.Get("active").(bool),
		Published:  d.Get("published").(bool),
		Algorithm:  d.Get("algorithm").(string),
		Bits:       d.Get("bits").(int),
		PrivateKey: d.Get("content").(string),
	}

	ctx = tflog.SetField(ctx, "zone_name", zone)
	tflog.Debug(ctx, "Creating PowerDNS cryptokey")

	created, err := client.PDNS.CreateCryptokey(ctx, zone, cryptokey)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(cryptokeyID(zone, created.ID))
	tflog.Info(ctx, "Created PowerDNS cryptokey", map[string]any{"key_id": created.ID})
	return resourcePDNSZoneCryptokeyRead(ctx, d, meta)
}

func resourcePDNSZoneCryptokeyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone, id, err := parseCryptokeyID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	ctx = tflog.SetField(ctx, "zone_name", zone)
	ctx = tflog.SetField(ctx, "key_id", id)
	tflog.Debug(ctx, "Reading PowerDNS cryptokey")

	cryptokey, err := client.PDNS.GetCryptokey(ctx, zone, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			tflog.Warn(ctx, "Cryptokey not found; removing from state")
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("couldn't fetch PowerDNS cryptokey: %w", err))
	}

	if err := d.Set("zone", zone); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS cryptokey zone: %w", err))
	}
	if err := d.Set("keytype", cryptokey.KeyType); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS cryptokey keytype: %w", err))
	}
	if err := d.Set("active", cryptokey.Active); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS cryptokey active: %w", err))
	}
	if err := d.Set("published", cryptokey.Published); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS cryptokey published: %w", err))
	}
	if err := d.Set("algorithm", cryptokey.Algorithm); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS cryptokey algorithm: %w", err))
	}
	if err := d.Set("bits", cryptokey.Bits); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS cryptokey bits: %w", err))
	}
	if err := d.Set("key_id", cryptokey.ID); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS cryptokey key ID: %w", err))
	}
	if err := d.Set("dnskey", cryptokey.DNSKey); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS cryptokey DNSKEY: %w", err))
	}
	if err := d.Set("ds", cryptokey.DS); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS cryptokey DS records: %w", err))
	}
	if err := d.Set("cds", cryptokey.CDS); err != nil {
		return diag.FromErr(fmt.Errorf("error setting PowerDNS cryptokey CDS records: %w", err))
	}

	return nil
}

func resourcePDNSZoneCryptokeyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone, id, err := parseCryptokeyID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	ctx = tflog.SetField(ctx, "zone_name", zone)
	ctx = tflog.SetField(ctx, "key_id", id)
	tflog.Debug(ctx, "Updating PowerDNS cryptokey")

	if d.HasChanges("active", "published") {
		// PowerDNS rejects an update that leaves out "active", so both flags go
		// out together even when only one of them changed.
		if err := client.PDNS.UpdateCryptokey(ctx, zone, id, d.Get("active").(bool), d.Get("published").(bool)); err != nil {
			return diag.FromErr(fmt.Errorf("error updating PowerDNS cryptokey: %w", err))
		}
	}

	return resourcePDNSZoneCryptokeyRead(ctx, d, meta)
}

func resourcePDNSZoneCryptokeyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*ProviderClients)

	zone, id, err := parseCryptokeyID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	ctx = tflog.SetField(ctx, "zone_name", zone)
	ctx = tflog.SetField(ctx, "key_id", id)
	tflog.Debug(ctx, "Deleting PowerDNS cryptokey")

	if err := client.PDNS.DeleteCryptokey(ctx, zone, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("error deleting PowerDNS cryptokey: %w", err))
	}

	tflog.Info(ctx, "Deleted PowerDNS cryptokey")
	return nil
}

func resourcePDNSZoneCryptokeyImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if _, _, err := parseCryptokeyID(d.Id()); err != nil {
		return nil, err
	}

	if diags := resourcePDNSZoneCryptokeyRead(ctx, d, meta); diags.HasError() {
		return nil, fmt.Errorf("error importing PowerDNS cryptokey: %s", diags[0].Summary)
	}

	return []*schema.ResourceData{d}, nil
}
