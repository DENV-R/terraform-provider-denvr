package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &redirectionsResource{}
	_ resource.ResourceWithConfigure   = &redirectionsResource{}
	_ resource.ResourceWithImportState = &redirectionsResource{}
)

// NewRedirectionsResource : la liste COMPLÈTE des ports publiés d'un espace (déclaratif, comme le socle).
func NewRedirectionsResource() resource.Resource {
	return &redirectionsResource{}
}

type redirectionsResource struct {
	client *Client
}

type redirectionModel struct {
	Proto      types.String `tfsdk:"proto"`
	PublicPort types.Int64  `tfsdk:"public_port"`
	TargetIP   types.String `tfsdk:"target_ip"`
	TargetPort types.Int64  `tfsdk:"target_port"`
}

type redirectionsResourceModel struct {
	ID           types.String       `tfsdk:"id"`
	EspaceID     types.String       `tfsdk:"espace_id"`
	Redirections []redirectionModel `tfsdk:"redirection"`
	Etat         types.String       `tfsdk:"etat"`
}

type redirectionAPI struct {
	Proto      string `json:"proto"`
	PublicPort int64  `json:"public_port"`
	TargetIP   string `json:"target_ip"`
	TargetPort int64  `json:"target_port"`
}

func (r *redirectionsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_redirections"
}

func (r *redirectionsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Les ports publiés depuis l'adresse publique d'un `denvr_espace` vers ses machines. DÉCLARATIF : cette ressource porte la " +
			"liste COMPLÈTE ; elle est envoyée entière au socle et appliquée à chaud sur les deux passerelles (moins d'une minute), ou refusée en bloc. " +
			"Une seule ressource par espace. `terraform destroy` referme tout (liste vide).\n\n" +
			"Règles du socle, vérifiées avant l'envoi : `tcp` ou `udp` ; ports 1 à 65535 ; le port public 22 jamais, ni 80, 443 et 6443 si l'espace " +
			"a un cluster ; cible dans le réseau privé de l'espace, de .20 à .249 ; un couple proto/port public une seule fois ; 20 au plus.",
		Attributes: map[string]schema.Attribute{
			"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"espace_id": schema.StringAttribute{Required: true, MarkdownDescription: "L'identifiant du `denvr_espace` (prêt).", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"etat":      schema.StringAttribute{Computed: true, MarkdownDescription: "`a_jour` quand le socle a confirmé la liste ; `en_cours` ; `erreur` (la liste précédente reste appliquée)."},
		},
		Blocks: map[string]schema.Block{
			"redirection": schema.ListNestedBlock{
				MarkdownDescription: "Une redirection : port public → machine, port.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"proto":       schema.StringAttribute{Required: true, MarkdownDescription: "`tcp` ou `udp`."},
						"public_port": schema.Int64Attribute{Required: true},
						"target_ip":   schema.StringAttribute{Required: true, MarkdownDescription: "L'adresse privée de la machine (sans le /24), ex. `10.41.0.20`."},
						"target_port": schema.Int64Attribute{Required: true},
					},
				},
			},
		},
	}
}

func (r *redirectionsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	if client, ok := req.ProviderData.(*Client); ok {
		r.client = client
	} else {
		resp.Diagnostics.AddError("Type de client inattendu", "Le provider n'a pas fourni un *Client.")
	}
}

func versAPI(liste []redirectionModel) []map[string]any {
	out := make([]map[string]any, 0, len(liste))
	for _, x := range liste {
		out = append(out, map[string]any{"proto": x.Proto.ValueString(), "public_port": x.PublicPort.ValueInt64(),
			"target_ip": x.TargetIP.ValueString(), "target_port": x.TargetPort.ValueInt64()})
	}
	return out
}

func depuisAPI(liste []redirectionAPI) []redirectionModel {
	out := make([]redirectionModel, 0, len(liste))
	for _, x := range liste {
		out = append(out, redirectionModel{Proto: types.StringValue(x.Proto), PublicPort: types.Int64Value(x.PublicPort),
			TargetIP: types.StringValue(x.TargetIP), TargetPort: types.Int64Value(x.TargetPort)})
	}
	return out
}

// appliquer : PUT de la liste complète, puis attente de la confirmation du socle (au plus 5 min).
func (r *redirectionsResource) appliquer(ctx context.Context, espaceID string, liste []redirectionModel) (espaceAPI, error) {
	var rep map[string]any
	if err := r.client.do("PUT", "/espaces/"+espaceID+"/redirections/", map[string]any{"redirections": versAPI(liste)}, &rep); err != nil {
		return espaceAPI{}, err
	}
	fin := time.Now().Add(5 * time.Minute)
	for {
		e, err := r.client.lireEspace(espaceID)
		if err != nil {
			return e, err
		}
		switch e.RedirEtat {
		case "a_jour":
			return e, nil
		case "erreur":
			return e, fmt.Errorf("le socle a refusé la liste : %s", e.Message)
		}
		if time.Now().After(fin) {
			return e, fmt.Errorf("le socle n'a pas confirmé la liste en 5 minutes (état %s)", e.RedirEtat)
		}
		select {
		case <-ctx.Done():
			return e, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func (r *redirectionsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan redirectionsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.appliquer(ctx, plan.EspaceID.ValueString(), plan.Redirections)
	if err != nil {
		resp.Diagnostics.AddError("Publication des ports impossible", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.EspaceID.ValueString())
	plan.Redirections = depuisAPI(e.Redirections)
	plan.Etat = types.StringValue(e.RedirEtat)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *redirectionsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state redirectionsResourceModel
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
	state.ID = types.StringValue(state.EspaceID.ValueString())
	state.Redirections = depuisAPI(e.Redirections)
	state.Etat = types.StringValue(e.RedirEtat)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *redirectionsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan redirectionsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	e, err := r.appliquer(ctx, plan.EspaceID.ValueString(), plan.Redirections)
	if err != nil {
		resp.Diagnostics.AddError("Publication des ports impossible", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.EspaceID.ValueString())
	plan.Redirections = depuisAPI(e.Redirections)
	plan.Etat = types.StringValue(e.RedirEtat)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *redirectionsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state redirectionsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.appliquer(ctx, state.EspaceID.ValueString(), nil); err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			return
		}
		resp.Diagnostics.AddError("Fermeture des ports impossible", err.Error())
	}
}

func (r *redirectionsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("espace_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
