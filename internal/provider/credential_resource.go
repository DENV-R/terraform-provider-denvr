package provider

import (
	"context"
	"errors"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &credentialResource{}
	_ resource.ResourceWithConfigure   = &credentialResource{}
	_ resource.ResourceWithImportState = &credentialResource{}
)

func NewCredentialResource() resource.Resource {
	return &credentialResource{}
}

type credentialResource struct {
	client *Client
}

type credentialResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Nom       types.String `tfsdk:"nom"`
	Type      types.String `tfsdk:"type"`
	Buckets   types.List   `tfsdk:"buckets"`
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
	Isolee    types.Bool   `tfsdk:"isolee"`
}

type credentialAPI struct {
	ID        int64   `json:"id"`
	Nom       string  `json:"nom"`
	Type      string  `json:"type"`
	AccessKey string  `json:"access_key"`
	SecretKey string  `json:"secret_key"`
	Isolee    bool    `json:"isolee"`
	Buckets   []int64 `json:"buckets_autorises"`
}

type credentialEnvelope struct {
	Status     string        `json:"status"`
	Credential credentialAPI `json:"credential"`
}

func (r *credentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_credential"
}

func (r *credentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Une clé d'accès S3 DENV-R (ROOT ou restreinte à des espaces).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"nom": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Nom lisible de la clé (modifiable).",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("root"),
				MarkdownDescription: "`root` (accès total) ou `restreinte` (limitée aux `buckets`). Non modifiable après création.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"buckets": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "IDs des espaces autorisés (pour une clé `restreinte`). Modifiable.",
			},
			"access_key": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifiant public de la clé.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"secret_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Clé secrète. Renvoyée UNE SEULE FOIS à la création, puis conservée dans l'état Terraform (protégez cet état).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"isolee": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Vraie isolation IAM active (compte multi-tenant).",
			},
		},
	}
}

func (r *credentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	if client, ok := req.ProviderData.(*Client); ok {
		r.client = client
	} else {
		resp.Diagnostics.AddError("Type de client inattendu", "Le provider n'a pas fourni un *Client.")
	}
}

// listeVersInts convertit la liste HCL des buckets en []int64 (nil si absente).
func listeVersInts(ctx context.Context, l types.List) ([]int64, diag.Diagnostics) {
	if l.IsNull() || l.IsUnknown() {
		return nil, nil
	}
	var ids []int64
	d := l.ElementsAs(ctx, &ids, false)
	return ids, d
}

func (m *credentialResourceModel) applique(ctx context.Context, c credentialAPI, avecSecret bool) diag.Diagnostics {
	m.ID = types.StringValue(strconv.FormatInt(c.ID, 10))
	m.Nom = types.StringValue(c.Nom)
	m.Type = types.StringValue(c.Type)
	m.AccessKey = types.StringValue(c.AccessKey)
	m.Isolee = types.BoolValue(c.Isolee)
	if avecSecret {
		m.SecretKey = types.StringValue(c.SecretKey) // uniquement à la création
	}
	liste, d := types.ListValueFrom(ctx, types.Int64Type, c.Buckets)
	m.Buckets = liste
	return d
}

func (r *credentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan credentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"nom": plan.Nom.ValueString(), "type": plan.Type.ValueString()}
	ids, d := listeVersInts(ctx, plan.Buckets)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Type.ValueString() == "restreinte" && ids != nil {
		body["buckets"] = ids
	}
	var env credentialEnvelope
	if err := r.client.do("POST", "/credentials/", body, &env); err != nil {
		resp.Diagnostics.AddError("Création de la clé impossible", err.Error())
		return
	}
	resp.Diagnostics.Append(plan.applique(ctx, env.Credential, true)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *credentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state credentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var env credentialEnvelope
	err := r.client.do("GET", "/credentials/"+state.ID.ValueString()+"/", nil, &env)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Lecture de la clé impossible", err.Error())
		return
	}
	resp.Diagnostics.Append(state.applique(ctx, env.Credential, false)...) // le secret n'est jamais relu : on conserve l'état
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *credentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan credentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Conserve le secret déjà en état (l'API ne le renvoie plus).
	var state credentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"nom": plan.Nom.ValueString()}
	ids, d := listeVersInts(ctx, plan.Buckets)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if ids != nil {
		body["buckets"] = ids
	}
	var env credentialEnvelope
	if err := r.client.do("PATCH", "/credentials/"+plan.ID.ValueString()+"/", body, &env); err != nil {
		resp.Diagnostics.AddError("Mise à jour de la clé impossible", err.Error())
		return
	}
	resp.Diagnostics.Append(plan.applique(ctx, env.Credential, false)...)
	plan.SecretKey = state.SecretKey
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *credentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state credentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.do("DELETE", "/credentials/"+state.ID.ValueString()+"/", nil, nil); err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			return
		}
		resp.Diagnostics.AddError("Révocation de la clé impossible", err.Error())
	}
}

func (r *credentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
