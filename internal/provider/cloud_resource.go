package provider

import (
	"context"
	"errors"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &cloudResource{}
	_ resource.ResourceWithConfigure   = &cloudResource{}
	_ resource.ResourceWithImportState = &cloudResource{}
)

func NewCloudResource() resource.Resource {
	return &cloudResource{}
}

type cloudResource struct {
	client *Client
}

type cloudResourceModel struct {
	ID           types.String `tfsdk:"id"`
	ProviderType types.String `tfsdk:"provider_type"`
	Nom          types.String `tfsdk:"nom"`
	Endpoint     types.String `tfsdk:"endpoint"`
	Region       types.String `tfsdk:"region"`
	AccessKey    types.String `tfsdk:"access_key"`
	SecretKey    types.String `tfsdk:"secret_key"`
	VerifyTLS    types.Bool   `tfsdk:"verify_tls"`
	LectureSeule types.Bool   `tfsdk:"lecture_seule"`
}

type cloudAPI struct {
	ID           int64  `json:"id"`
	Provider     string `json:"provider"`
	Nom          string `json:"nom"`
	Endpoint     string `json:"endpoint"`
	Region       string `json:"region"`
	AccessKey    string `json:"access_key"`
	VerifyTLS    bool   `json:"verify_tls"`
	LectureSeule bool   `json:"lecture_seule"`
}

type cloudEnvelope struct {
	Status string   `json:"status"`
	Cloud  cloudAPI `json:"cloud"`
}

func (r *cloudResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud"
}

func (r *cloudResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Une connexion à un stockage S3 externe (provider ou on-premise).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"provider_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Fournisseur : `ovh`, `aws`, `wasabi`, `backblaze`, `gcs`, `scaleway`, `minio` ou `autre`. Non modifiable après création.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"nom": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Nom lisible (ex. « OVH prod Gravelines »).",
			},
			"endpoint": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Endpoint S3 (ex. `s3.gra.io.cloud.ovh.net`).",
			},
			"region": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Région (ex. `gra`, `eu-west-3`).",
			},
			"access_key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Identifiant d'accès du compte externe.",
			},
			"secret_key": schema.StringAttribute{
				Required:            true,
				Sensitive:           true,
				MarkdownDescription: "Secret du compte externe. Conservé dans l'état Terraform (protégez cet état) ; jamais relu depuis l'API.",
			},
			"verify_tls": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Vérifier le certificat TLS. Mettre `false` pour un on-premise à certificat auto-signé.",
			},
			"lecture_seule": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Connexion en lecture seule. `false` autorise l'écriture (migration vers ce cloud).",
			},
		},
	}
}

func (r *cloudResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	if client, ok := req.ProviderData.(*Client); ok {
		r.client = client
	} else {
		resp.Diagnostics.AddError("Type de client inattendu", "Le provider n'a pas fourni un *Client.")
	}
}

// applique recopie la réponse de l'API dans le modèle SANS toucher au secret (jamais renvoyé).
func (m *cloudResourceModel) applique(c cloudAPI) {
	m.ID = types.StringValue(strconv.FormatInt(c.ID, 10))
	m.ProviderType = types.StringValue(c.Provider)
	m.Nom = types.StringValue(c.Nom)
	m.Endpoint = types.StringValue(c.Endpoint)
	m.Region = types.StringValue(c.Region)
	m.AccessKey = types.StringValue(c.AccessKey)
	m.VerifyTLS = types.BoolValue(c.VerifyTLS)
	m.LectureSeule = types.BoolValue(c.LectureSeule)
}

func (r *cloudResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cloudResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{
		"provider": plan.ProviderType.ValueString(), "endpoint": plan.Endpoint.ValueString(),
		"region": plan.Region.ValueString(), "access_key": plan.AccessKey.ValueString(),
		"secret_key": plan.SecretKey.ValueString(), "nom": plan.Nom.ValueString(),
		"verify_tls": plan.VerifyTLS.ValueBool(), "lecture_seule": plan.LectureSeule.ValueBool(),
	}
	var env cloudEnvelope
	if err := r.client.do("POST", "/clouds/", body, &env); err != nil {
		resp.Diagnostics.AddError("Connexion du cloud impossible", err.Error())
		return
	}
	secret := plan.SecretKey
	plan.applique(env.Cloud)
	plan.SecretKey = secret // conservé depuis la configuration
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cloudResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cloudResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var env cloudEnvelope
	err := r.client.do("GET", "/clouds/"+state.ID.ValueString()+"/", nil, &env)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Lecture du cloud impossible", err.Error())
		return
	}
	secret := state.SecretKey
	state.applique(env.Cloud)
	state.SecretKey = secret // l'API ne renvoie jamais le secret : on conserve l'état
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cloudResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan cloudResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{
		"endpoint": plan.Endpoint.ValueString(), "region": plan.Region.ValueString(),
		"nom": plan.Nom.ValueString(), "access_key": plan.AccessKey.ValueString(),
		"secret_key": plan.SecretKey.ValueString(),
		"verify_tls": plan.VerifyTLS.ValueBool(), "lecture_seule": plan.LectureSeule.ValueBool(),
	}
	var env cloudEnvelope
	if err := r.client.do("PATCH", "/clouds/"+plan.ID.ValueString()+"/", body, &env); err != nil {
		resp.Diagnostics.AddError("Mise à jour du cloud impossible", err.Error())
		return
	}
	secret := plan.SecretKey
	plan.applique(env.Cloud)
	plan.SecretKey = secret
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *cloudResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cloudResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.do("DELETE", "/clouds/"+state.ID.ValueString()+"/", nil, nil); err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			return
		}
		resp.Diagnostics.AddError("Déconnexion du cloud impossible", err.Error())
	}
}

func (r *cloudResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
