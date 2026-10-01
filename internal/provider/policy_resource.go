package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &policyResource{}
	_ resource.ResourceWithConfigure   = &policyResource{}
	_ resource.ResourceWithImportState = &policyResource{}
)

func NewPolicyResource() resource.Resource {
	return &policyResource{}
}

type policyResource struct {
	client *Client
}

type policyResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Type   types.String `tfsdk:"type"`
	Actif  types.Bool   `tfsdk:"actif"`
	Params types.String `tfsdk:"params"`
}

type policyAPI struct {
	Type   string          `json:"type"`
	Nom    string          `json:"nom"`
	Actif  bool            `json:"actif"`
	Params json.RawMessage `json:"params"`
}

type policyEnvelope struct {
	Status   string      `json:"status"`
	Policy   policyAPI   `json:"policy"`
	Policies []policyAPI `json:"policies"`
}

func (r *policyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (r *policyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Une politique du control plane (intention déclarée). Une seule par type et par compte.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "`retention`, `souverainete` ou `immutabilite`. Identifie la politique (non modifiable).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"actif": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Politique active.",
			},
			"params": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Paramètres en JSON (ex. `jsonencode({ min_jours = 90 })` pour `retention`, `jsonencode({ zones = [\"FR\",\"EU\"] })` pour `souverainete`).",
			},
		},
	}
}

func (r *policyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	if client, ok := req.ProviderData.(*Client); ok {
		r.client = client
	} else {
		resp.Diagnostics.AddError("Type de client inattendu", "Le provider n'a pas fourni un *Client.")
	}
}

// applique recopie la politique de l'API dans le modèle. Les params sont re-normalisés en JSON
// compact à clés triées pour coller à la sortie de jsonencode (évite un diff Terraform perpétuel).
func (m *policyResourceModel) applique(p policyAPI) error {
	m.ID = types.StringValue(p.Type)
	m.Type = types.StringValue(p.Type)
	m.Actif = types.BoolValue(p.Actif)
	if len(p.Params) == 0 {
		m.Params = types.StringValue("{}")
		return nil
	}
	var obj any
	if err := json.Unmarshal(p.Params, &obj); err != nil {
		return err
	}
	compact, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	m.Params = types.StringValue(string(compact))
	return nil
}

// corps construit le corps du PUT (type + actif + params décodés depuis la chaîne JSON).
func (m *policyResourceModel) corps() (map[string]any, error) {
	body := map[string]any{"type": m.Type.ValueString(), "actif": m.Actif.ValueBool()}
	if !m.Params.IsNull() && !m.Params.IsUnknown() && m.Params.ValueString() != "" {
		var obj any
		if err := json.Unmarshal([]byte(m.Params.ValueString()), &obj); err != nil {
			return nil, fmt.Errorf("`params` doit être du JSON valide : %w", err)
		}
		body["params"] = obj
	}
	return body, nil
}

func (r *policyResource) ecrire(m *policyResourceModel) (policyAPI, error) {
	body, err := m.corps()
	if err != nil {
		return policyAPI{}, err
	}
	var env policyEnvelope
	if err := r.client.do("PUT", "/policies/", body, &env); err != nil {
		return policyAPI{}, err
	}
	return env.Policy, nil
}

func (r *policyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pol, err := r.ecrire(&plan)
	if err != nil {
		resp.Diagnostics.AddError("Déclaration de la politique impossible", err.Error())
		return
	}
	if err := plan.applique(pol); err != nil {
		resp.Diagnostics.AddError("Réponse de politique illisible", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *policyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state policyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Pas de lecture par id : on liste et on retrouve la politique par son type.
	var env policyEnvelope
	if err := r.client.do("GET", "/policies/", nil, &env); err != nil {
		resp.Diagnostics.AddError("Lecture des politiques impossible", err.Error())
		return
	}
	cible := state.Type.ValueString()
	for _, p := range env.Policies {
		if p.Type == cible {
			if err := state.applique(p); err != nil {
				resp.Diagnostics.AddError("Réponse de politique illisible", err.Error())
				return
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	// Type absent de la liste (ne devrait pas arriver : les types sont fixes) : on retire.
	resp.State.RemoveResource(ctx)
}

func (r *policyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan policyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pol, err := r.ecrire(&plan)
	if err != nil {
		resp.Diagnostics.AddError("Mise à jour de la politique impossible", err.Error())
		return
	}
	if err := plan.applique(pol); err != nil {
		resp.Diagnostics.AddError("Réponse de politique illisible", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *policyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// L'API ne supprime pas une politique (types fixes) : « supprimer » = désactiver.
	body := map[string]any{"type": state.Type.ValueString(), "actif": false}
	var env policyEnvelope
	if err := r.client.do("PUT", "/policies/", body, &env); err != nil {
		resp.Diagnostics.AddError("Désactivation de la politique impossible", err.Error())
	}
}

func (r *policyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// L'identifiant importé est le type de la politique (ex. `retention`).
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
