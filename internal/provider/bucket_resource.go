package provider

import (
	"context"
	"errors"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &bucketResource{}
	_ resource.ResourceWithConfigure   = &bucketResource{}
	_ resource.ResourceWithImportState = &bucketResource{}
)

// NewBucketResource est la fabrique enregistrée dans le provider.
func NewBucketResource() resource.Resource {
	return &bucketResource{}
}

type bucketResource struct {
	client *Client
}

type bucketResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Nom             types.String `tfsdk:"nom"`
	Offre           types.String `tfsdk:"offre"`
	TailleTo        types.Int64  `tfsdk:"taille_to"`
	Versioning      types.Bool   `tfsdk:"versioning"`
	Tags            types.String `tfsdk:"tags"`
	Multisite       types.Bool   `tfsdk:"multisite"`
	ObjectLockDays  types.Int64  `tfsdk:"object_locking_days"`
	ObjectLockMode  types.String `tfsdk:"object_lock_mode"`
	ImmutabiliteApp types.Bool   `tfsdk:"immutabilite_app"`
}

// Formes de réponse de l'API (superset création + lecture : les noms de champs diffèrent).
type bucketAPI struct {
	ID                int64  `json:"id"`
	BucketID          int64  `json:"bucket_id"`
	Nom               string `json:"nom"`
	Offre             string `json:"offre"`
	OffreCode         string `json:"offre_code"`
	TailleLimiteTo    int64  `json:"taille_limite_to"`
	TailleAllouee     int64  `json:"taille_allouee_to"`
	Versioning        bool   `json:"versioning"`
	Multisite         bool   `json:"multisite"`
	ObjectLockJours   int64  `json:"object_lock_jours"`
	ObjectLockDays    int64  `json:"object_locking_days"`
	ObjectLockMode    string `json:"object_lock_mode"`
	ObjectLockEnabled bool   `json:"object_lock_enabled"`
	Tags              string `json:"tags"`
}

type bucketEnvelope struct {
	Status string    `json:"status"`
	Bucket bucketAPI `json:"bucket"`
}

func (r *bucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_bucket"
}

func (r *bucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Un espace de stockage S3 DENV-R.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Identifiant interne de l'espace.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"nom": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Nom de l'espace (3 à 63 car. : minuscules, chiffres, tirets). Non modifiable après création.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"offre": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Code de l'offre (modifiable).",
			},
			"taille_to": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Quota alloué en To.",
			},
			"versioning": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Versioning activé.",
			},
			"tags": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Tags libres.",
			},
			"multisite": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Réplication multi-sites. Non modifiable après création.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"object_locking_days": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Verrouillage WORM en jours (0 = désactivé). Non modifiable après création.",
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"object_lock_mode": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Mode WORM : GOVERNANCE (défaut) ou COMPLIANCE (irréversible). Non modifiable après création.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"immutabilite_app": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Immuabilité gérée par votre logiciel de sauvegarde (Veeam, Commvault, Rubrik…) : Object Lock activé SANS rétention par défaut. Exclusif avec object_locking_days. Non modifiable après création.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *bucketResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Type de client inattendu", "Le provider n'a pas fourni un *Client.")
		return
	}
	r.client = client
}

// applique recopie la réponse de l'API dans le modèle Terraform (normalise création vs lecture).
func (m *bucketResourceModel) applique(b bucketAPI) {
	id := b.ID
	if id == 0 {
		id = b.BucketID
	}
	m.ID = types.StringValue(strconv.FormatInt(id, 10))
	m.Nom = types.StringValue(b.Nom)

	offre := b.Offre
	if offre == "" {
		offre = b.OffreCode
	}
	m.Offre = types.StringValue(offre)

	taille := b.TailleLimiteTo
	if taille == 0 {
		taille = b.TailleAllouee
	}
	m.TailleTo = types.Int64Value(taille)

	m.Versioning = types.BoolValue(b.Versioning)
	m.Tags = types.StringValue(b.Tags)
	m.Multisite = types.BoolValue(b.Multisite)

	lock := b.ObjectLockJours
	if lock == 0 {
		lock = b.ObjectLockDays
	}
	m.ObjectLockDays = types.Int64Value(lock)

	if b.ObjectLockMode != "" {
		m.ObjectLockMode = types.StringValue(b.ObjectLockMode)
	} else {
		m.ObjectLockMode = types.StringNull()
	}

	// Immuabilité « gérée par l'app » = Object Lock activé, mode APP, sans rétention par défaut.
	m.ImmutabiliteApp = types.BoolValue(b.ObjectLockMode == "APP" || (b.ObjectLockEnabled && lock == 0 && b.ObjectLockMode == ""))
}

func (r *bucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan bucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]any{"nom": plan.Nom.ValueString(), "offre": plan.Offre.ValueString()}
	if !plan.TailleTo.IsNull() && !plan.TailleTo.IsUnknown() {
		body["taille_to"] = plan.TailleTo.ValueInt64()
	}
	if !plan.Versioning.IsNull() && !plan.Versioning.IsUnknown() {
		body["versioning"] = plan.Versioning.ValueBool()
	}
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		body["tags"] = plan.Tags.ValueString()
	}
	if !plan.Multisite.IsNull() && !plan.Multisite.IsUnknown() {
		body["multisite"] = plan.Multisite.ValueBool()
	}
	if !plan.ObjectLockDays.IsNull() && !plan.ObjectLockDays.IsUnknown() {
		body["object_locking_days"] = plan.ObjectLockDays.ValueInt64()
	}
	if !plan.ObjectLockMode.IsNull() && !plan.ObjectLockMode.IsUnknown() {
		body["object_lock_mode"] = plan.ObjectLockMode.ValueString()
	}
	if !plan.ImmutabiliteApp.IsNull() && !plan.ImmutabiliteApp.IsUnknown() && plan.ImmutabiliteApp.ValueBool() {
		body["immutabilite_app"] = true
	}

	var env bucketEnvelope
	if err := r.client.do("POST", "/buckets/", body, &env); err != nil {
		resp.Diagnostics.AddError("Création de l'espace impossible", err.Error())
		return
	}
	plan.applique(env.Bucket)

	// Rafraîchit depuis l'état réel (renseigne les champs calculés côté serveur).
	if _, err := r.refresh(&plan); err != nil {
		resp.Diagnostics.AddError("Relecture après création impossible", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state bucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.refresh(&state)
	if err != nil {
		resp.Diagnostics.AddError("Lecture de l'espace impossible", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx) // supprimé hors Terraform : on le retire de l'état.
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *bucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan bucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := plan.ID.ValueString()

	body := map[string]any{"offre": plan.Offre.ValueString()}
	if !plan.TailleTo.IsNull() && !plan.TailleTo.IsUnknown() {
		body["taille_limite_to"] = plan.TailleTo.ValueInt64()
	}
	if !plan.Versioning.IsNull() && !plan.Versioning.IsUnknown() {
		body["versioning"] = plan.Versioning.ValueBool()
	}
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		body["tags"] = plan.Tags.ValueString()
	}
	var env bucketEnvelope
	if err := r.client.do("PATCH", "/buckets/"+id+"/", body, &env); err != nil {
		resp.Diagnostics.AddError("Mise à jour de l'espace impossible", err.Error())
		return
	}
	if _, err := r.refresh(&plan); err != nil {
		resp.Diagnostics.AddError("Relecture après mise à jour impossible", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *bucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state bucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.do("DELETE", "/buckets/"+state.ID.ValueString()+"/", nil, nil)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			return // déjà absent : rien à faire.
		}
		resp.Diagnostics.AddError("Suppression de l'espace impossible", err.Error())
	}
}

func (r *bucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// refresh relit l'espace par son ID et met à jour le modèle. found=false sur 404.
func (r *bucketResource) refresh(m *bucketResourceModel) (bool, error) {
	var env bucketEnvelope
	err := r.client.do("GET", "/buckets/"+m.ID.ValueString()+"/", nil, &env)
	if err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == 404 {
			return false, nil
		}
		return false, err
	}
	m.applique(env.Bucket)
	return true, nil
}
