package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &machineResource{}
	_ resource.ResourceWithConfigure   = &machineResource{}
	_ resource.ResourceWithImportState = &machineResource{}
)

// NewMachineResource : une machine virtuelle nue dans le réseau privé d'un espace DENV-R.
func NewMachineResource() resource.Resource {
	return &machineResource{}
}

type machineResource struct {
	client *Client
}

type machineResourceModel struct {
	ID        types.String `tfsdk:"id"`
	EspaceID  types.String `tfsdk:"espace_id"`
	Modele    types.String `tfsdk:"modele"`
	Hostname  types.String `tfsdk:"hostname"`
	Coeurs    types.Int64  `tfsdk:"coeurs"`
	MemoireMo types.Int64  `tfsdk:"memoire_mo"`
	DisqueGo  types.Int64  `tfsdk:"disque_go"`
	ClesSSH   types.String `tfsdk:"cles_ssh"`
	VMID      types.String `tfsdk:"vmid"`
	IPPrivee  types.String `tfsdk:"ip_privee"`
	Etat      types.String `tfsdk:"etat"`
}

type machineAPI struct {
	ID        int64  `json:"id"`
	Hostname  string `json:"hostname"`
	VMID      string `json:"vmid"`
	Systeme   string `json:"systeme"`
	IPPrivee  string `json:"ip_privee"`
	Coeurs    int64  `json:"coeurs"`
	MemoireMo int64  `json:"memoire_mo"`
	DisqueGo  int64  `json:"disque_go"`
	Etat      string `json:"etat"`
}

type machineEnvelope struct {
	Status  string     `json:"status"`
	Message string     `json:"message"`
	Machine machineAPI `json:"machine"`
}

func (r *machineResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine"
}

func (r *machineResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Une machine virtuelle nue dans le réseau privé d'un `denvr_espace` : la console clone un modèle cloud-init sur le socle, " +
			"lui donne une adresse privée, votre clé SSH et la taille choisie, puis la démarre. Facturée au prix de Paramètres à partir du mois suivant sa création (le mois entamé n'est pas facturé) " +
			"(3 €/vCPU, 1 €/Go de mémoire, 0,05 €/Go de disque par mois, tels que lus sur la console).\n\n" +
			"Depuis Internet, seuls les ports publiés via `denvr_redirections` entrent ; le port 22 n'entre jamais (SSH depuis le réseau privé).\n\n" +
			"Toute modification de taille ou de système recrée la machine. `terraform destroy` retire la machine de l'état mais NE la détruit PAS sur le socle : " +
			"la console n'appelle jamais une suppression de machine ; demandez-le à l'équipe (ou fermez l'espace).",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"espace_id": schema.StringAttribute{Required: true, MarkdownDescription: "L'identifiant du `denvr_espace` (prêt).", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"modele": schema.StringAttribute{Required: true, MarkdownDescription: "`debian12`, `ubuntu2404` ou `alpine324`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"hostname": schema.StringAttribute{Required: true, MarkdownDescription: "Lettres minuscules, chiffres et tirets.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"coeurs":   schema.Int64Attribute{Required: true, MarkdownDescription: "1 à 8 vCPU.", PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
			"memoire_mo": schema.Int64Attribute{Required: true, MarkdownDescription: "1024 à 32768 Mo.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
			"disque_go": schema.Int64Attribute{Required: true, MarkdownDescription: "10 à 200 Go.", PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
			"cles_ssh": schema.StringAttribute{Required: true, MarkdownDescription: "Votre clé SSH publique (`ssh-ed25519 …`) : la seule porte dans la machine.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"vmid":      schema.StringAttribute{Computed: true, MarkdownDescription: "L'identifiant de la machine sur le socle."},
			"ip_privee": schema.StringAttribute{Computed: true, MarkdownDescription: "L'adresse privée attribuée, ex. `10.41.0.20/24` : la cible d'une redirection."},
			"etat":      schema.StringAttribute{Computed: true, MarkdownDescription: "`cree` (créée, démarrée et relue), `en_cours`, `erreur`, `retiree`."},
		},
	}
}

func (r *machineResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	if client, ok := req.ProviderData.(*Client); ok {
		r.client = client
	} else {
		resp.Diagnostics.AddError("Type de client inattendu", "Le provider n'a pas fourni un *Client.")
	}
}

func (m *machineResourceModel) applique(a machineAPI) {
	m.ID = types.StringValue(strconv.FormatInt(a.ID, 10))
	m.Hostname = types.StringValue(a.Hostname)
	m.VMID = types.StringValue(a.VMID)
	m.IPPrivee = types.StringValue(a.IPPrivee)
	m.Etat = types.StringValue(a.Etat)
	if a.Systeme != "" {
		m.Modele = types.StringValue(a.Systeme)
	}
	m.Coeurs = types.Int64Value(a.Coeurs)
	m.MemoireMo = types.Int64Value(a.MemoireMo)
	m.DisqueGo = types.Int64Value(a.DisqueGo)
}

func (r *machineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan machineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{
		"modele": plan.Modele.ValueString(), "hostname": plan.Hostname.ValueString(), "cles_ssh": plan.ClesSSH.ValueString(),
		"coeurs": plan.Coeurs.ValueInt64(), "memoire_mo": plan.MemoireMo.ValueInt64(), "disque_go": plan.DisqueGo.ValueInt64(),
	}
	var env machineEnvelope
	if err := r.client.do("POST", "/espaces/"+plan.EspaceID.ValueString()+"/machines/", body, &env); err != nil {
		resp.Diagnostics.AddError("Création de la machine refusée", err.Error())
		return
	}
	cles := plan.ClesSSH
	plan.applique(env.Machine)
	plan.ClesSSH = cles
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *machineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state machineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.client.lireEspace(state.EspaceID.ValueString())
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Lecture de l'espace impossible", err.Error())
		return
	}
	id, _ := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	for _, m := range e.Machines {
		if m.ID == id {
			cles := state.ClesSSH
			state.applique(m)
			state.ClesSSH = cles
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx) // retirée côté console (espace fermé, machine détruite par l'équipe)
}

func (r *machineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Tous les attributs imposent un remplacement : rien à mettre à jour ici.
	var plan machineResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *machineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state machineResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.AddWarning("Machine retirée de l'état, pas du socle",
		fmt.Sprintf("La console n'appelle jamais une suppression de machine sur le socle : « %s » (vmid %s) reste en place et facturée. Demandez sa destruction à l'équipe DENV-R, ou fermez l'espace.",
			state.Hostname.ValueString(), state.VMID.ValueString()))
}

func (r *machineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import : « <espace_id>/<machine_id> ».
	for i, c := range req.ID {
		if c == '/' {
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("espace_id"), req.ID[:i])...)
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID[i+1:])...)
			return
		}
	}
	resp.Diagnostics.AddError("Identifiant d'import attendu : <espace_id>/<machine_id>", req.ID)
}
