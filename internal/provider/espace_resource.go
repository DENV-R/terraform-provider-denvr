package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &espaceResource{}
	_ resource.ResourceWithConfigure   = &espaceResource{}
	_ resource.ResourceWithImportState = &espaceResource{}
)

// NewEspaceResource : un espace du client sur le socle mutualisé DENV-R (réseau privé + cluster selon la taille).
func NewEspaceResource() resource.Resource {
	return &espaceResource{}
}

type espaceResource struct {
	client *Client
}

type espaceResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Code            types.String `tfsdk:"code"`
	Taille          types.String `tfsdk:"taille"`
	Etat            types.String `tfsdk:"etat"`
	Message         types.String `tfsdk:"message"`
	AdressePublique types.String `tfsdk:"adresse_publique"`
	VPC             types.String `tfsdk:"vpc"`
	VLAN            types.Int64  `tfsdk:"vlan"`
	Cluster         types.Bool   `tfsdk:"cluster"`
	ConnexionID     types.Int64  `tfsdk:"connexion_id"`
}

// espaceAPI : ce que l'API renvoie pour un espace (sous-ensemble utile au provider).
type espaceAPI struct {
	ID              int64            `json:"id"`
	Code            string           `json:"code"`
	Taille          string           `json:"taille"`
	Etat            string           `json:"etat"`
	Message         string           `json:"message"`
	AdressePublique *string          `json:"adresse_publique"`
	VLAN            *int64           `json:"vlan"`
	VPC             string           `json:"vpc"`
	Cluster         bool             `json:"cluster"`
	ConnexionID     *int64           `json:"connexion_id"`
	Redirections    []redirectionAPI `json:"redirections"`
	RedirEtat       string           `json:"redirections_etat"`
	Machines        []machineAPI     `json:"machines"`
}

type espaceEnvelope struct {
	Status string    `json:"status"`
	Espace espaceAPI `json:"espace"`
}

func (r *espaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_espace"
}

func (r *espaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Un espace sur le socle mutualisé DENV-R (Nantes) : un réseau privé à vous (VLAN, deux passerelles, une adresse publique) " +
			"et, selon la taille, un cluster Kubernetes de trois nœuds. La console demande la création au socle et la suit ; la ressource est " +
			"« appliquée » quand l'espace est PRÊT (dix minutes au plus pour un cluster). Facturé à l'heure, de la mise à disposition à la fermeture, sur la facture du mois suivant.\n\n" +
			"`terraform destroy` ne détruit rien lui-même : il DEMANDE la fermeture (ticket à l'équipe DENV-R, qui détruit sur le socle ; " +
			"la facturation s'arrête quand c'est fait). La console ne détient aucun pouvoir de suppression sur le socle, c'est voulu.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"code": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Le nom de l'espace sur le socle : 3 à 16 caractères, minuscules et chiffres, commençant par une lettre. Non modifiable.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"taille": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "`S`, `M`, `L` (cluster Kubernetes de 3 nœuds) ou `VPC` (réseau privé seul). Les tailles au tarif se lisent sur `GET /api/v1/espaces/`. Non modifiable.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"etat":             schema.StringAttribute{Computed: true, MarkdownDescription: "`pret` une fois appliqué ; `fermeture` après destroy ; `retire` quand l'équipe a détruit."},
			"message":          schema.StringAttribute{Computed: true, MarkdownDescription: "Ce que la console dit de l'espace, tel quel."},
			"adresse_publique": schema.StringAttribute{Computed: true, MarkdownDescription: "L'adresse IPv4 publique de l'espace (80, 443 et 6443 publiés vers le cluster ; les autres ports via `denvr_redirections`)."},
			"vpc":              schema.StringAttribute{Computed: true, MarkdownDescription: "Le réseau privé, ex. `10.41.0.0/24`."},
			"vlan":             schema.Int64Attribute{Computed: true},
			"cluster":          schema.BoolAttribute{Computed: true, MarkdownDescription: "Vrai si l'espace porte un cluster Kubernetes."},
			"connexion_id": schema.Int64Attribute{Computed: true,
				MarkdownDescription: "L'identifiant de la connexion Kubernetes posée par la console sur le cluster (déployez dessus avec `/api/v1/infrastructures/{id}/deploiements/`). Vide pour un réseau seul."},
		},
	}
}

func (r *espaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	if client, ok := req.ProviderData.(*Client); ok {
		r.client = client
	} else {
		resp.Diagnostics.AddError("Type de client inattendu", "Le provider n'a pas fourni un *Client.")
	}
}

func (m *espaceResourceModel) applique(e espaceAPI) {
	m.ID = types.StringValue(strconv.FormatInt(e.ID, 10))
	m.Code = types.StringValue(e.Code)
	m.Taille = types.StringValue(e.Taille)
	m.Etat = types.StringValue(e.Etat)
	m.Message = types.StringValue(e.Message)
	m.VPC = types.StringValue(e.VPC)
	m.Cluster = types.BoolValue(e.Cluster)
	if e.AdressePublique != nil {
		m.AdressePublique = types.StringValue(*e.AdressePublique)
	} else {
		m.AdressePublique = types.StringValue("")
	}
	if e.VLAN != nil {
		m.VLAN = types.Int64Value(*e.VLAN)
	} else {
		m.VLAN = types.Int64Null()
	}
	if e.ConnexionID != nil {
		m.ConnexionID = types.Int64Value(*e.ConnexionID)
	} else {
		m.ConnexionID = types.Int64Null()
	}
}

// lireEspace : GET d'un espace par identifiant.
func (c *Client) lireEspace(id string) (espaceAPI, error) {
	var env espaceEnvelope
	err := c.do("GET", "/espaces/"+id+"/", nil, &env)
	return env.Espace, err
}

// attendreEspace : relit l'espace toutes les 15 s jusqu'à `pret` (ou `erreur` → échec), au plus `max`.
func (c *Client) attendreEspace(ctx context.Context, id string, max time.Duration) (espaceAPI, error) {
	fin := time.Now().Add(max)
	for {
		e, err := c.lireEspace(id)
		if err != nil {
			return e, err
		}
		switch e.Etat {
		case "pret":
			return e, nil
		case "erreur":
			return e, fmt.Errorf("le socle a refusé ou échoué : %s", e.Message)
		}
		if time.Now().After(fin) {
			return e, fmt.Errorf("l'espace n'est pas prêt après %s (état %s : %s) ; relancez `terraform apply` plus tard, l'espace continue d'être suivi par la console", max, e.Etat, e.Message)
		}
		select {
		case <-ctx.Done():
			return e, ctx.Err()
		case <-time.After(15 * time.Second):
		}
	}
}

func (r *espaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan espaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var env espaceEnvelope
	body := map[string]any{"code": plan.Code.ValueString(), "taille": plan.Taille.ValueString()}
	if err := r.client.do("POST", "/espaces/", body, &env); err != nil {
		resp.Diagnostics.AddError("Demande d'espace refusée", err.Error())
		return
	}
	id := strconv.FormatInt(env.Espace.ID, 10)
	e, err := r.client.attendreEspace(ctx, id, 20*time.Minute)
	if err != nil {
		// L'espace existe côté console (en cours ou en erreur) : on le garde dans l'état pour ne pas le perdre de vue.
		plan.applique(e)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		resp.Diagnostics.AddError("Espace non prêt", err.Error())
		return
	}
	plan.applique(e)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *espaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state espaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.client.lireEspace(state.ID.ValueString())
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Lecture de l'espace impossible", err.Error())
		return
	}
	if e.Etat == "retire" {
		// Détruit sur le socle par l'équipe : la ressource n'existe plus.
		resp.State.RemoveResource(ctx)
		return
	}
	state.applique(e)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *espaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Code et taille imposent un remplacement ; rien d'autre n'est modifiable : on relit simplement.
	var plan espaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.client.lireEspace(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Lecture de l'espace impossible", err.Error())
		return
	}
	plan.applique(e)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *espaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state espaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var env espaceEnvelope
	err := r.client.do("DELETE", "/espaces/"+state.ID.ValueString()+"/", nil, &env)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			return
		}
		if errors.As(err, &apiErr) && apiErr.Status == 400 {
			// Déjà en fermeture, ou pas encore prêt : on le dit, et on retire de l'état (la console suit l'espace).
			resp.Diagnostics.AddWarning("Fermeture non engagée par cette commande", apiErr.Message)
			return
		}
		resp.Diagnostics.AddError("Demande de fermeture impossible", err.Error())
		return
	}
	resp.Diagnostics.AddWarning("Fermeture demandée, pas encore effective",
		fmt.Sprintf("L'espace « %s » reste en place (et facturé) jusqu'à ce que l'équipe DENV-R le détruise sur le socle ; vous serez prévenu par courriel. %s",
			state.Code.ValueString(), env.Espace.Message))
}

func (r *espaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
