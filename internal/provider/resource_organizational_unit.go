package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/bjoernf73/dry.module.ad/tf/terraform-provider-dryad/internal/ad"
	"github.com/bjoernf73/dry.module.ad/tf/terraform-provider-dryad/internal/client"
)

var (
	_ resource.Resource                = &organizationalUnitResource{}
	_ resource.ResourceWithConfigure   = &organizationalUnitResource{}
	_ resource.ResourceWithImportState = &organizationalUnitResource{}
)

func NewOrganizationalUnitResource() resource.Resource {
	return &organizationalUnitResource{}
}

type organizationalUnitResource struct {
	client *client.Client
}

type organizationalUnitResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Path              types.String `tfsdk:"path"`
	Description       types.String `tfsdk:"description"`
	DeleteSubtree     types.Bool   `tfsdk:"delete_subtree"`
	DistinguishedName types.String `tfsdk:"distinguished_name"`
	Name              types.String `tfsdk:"name"`
}

func (r *organizationalUnitResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organizational_unit"
}

func (r *organizationalUnitResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Active Directory organizational unit by executing PowerShell on a remote Windows host.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform resource identifier. Equals the organizational unit distinguished name.",
			},
			"path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Slash-delimited OU path relative to the domain root, for example `Contoso/Servers/Windows`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OU description.",
			},
			"delete_subtree": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Recursively delete child objects when destroying this resource.",
			},
			"distinguished_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Distinguished name of the organizational unit.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Leaf organizational unit name.",
			},
		},
	}
}

func (r *organizationalUnitResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	dryadClient, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}

	r.client = dryadClient
}

func (r *organizationalUnitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationalUnitResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ou, err := ad.EnsureOrganizationalUnit(ctx, r.client, plan.Path.ValueString(), optionalString(plan.Description))
	if err != nil {
		resp.Diagnostics.AddError("Unable to create organizational unit", err.Error())
		return
	}

	state := organizationalUnitResourceModel{
		ID:                types.StringValue(ou.DistinguishedName),
		Path:              types.StringValue(ou.Path),
		DeleteSubtree:     plan.DeleteSubtree,
		DistinguishedName: types.StringValue(ou.DistinguishedName),
		Name:              types.StringValue(ou.Name),
		Description:       stringPointerToTerraform(ou.Description),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *organizationalUnitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state organizationalUnitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	distinguishedName := state.DistinguishedName.ValueString()
	if distinguishedName == "" {
		distinguishedName = state.ID.ValueString()
	}

	ou, err := ad.ReadOrganizationalUnit(ctx, r.client, distinguishedName)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read organizational unit", err.Error())
		return
	}

	if !ou.Exists {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(ou.DistinguishedName)
	state.Path = types.StringValue(ou.Path)
	state.DistinguishedName = types.StringValue(ou.DistinguishedName)
	state.Name = types.StringValue(ou.Name)
	state.Description = stringPointerToTerraform(ou.Description)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *organizationalUnitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan organizationalUnitResourceModel
	var state organizationalUnitResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ou, err := ad.UpdateOrganizationalUnitDescription(ctx, r.client, state.DistinguishedName.ValueString(), optionalString(plan.Description))
	if err != nil {
		resp.Diagnostics.AddError("Unable to update organizational unit", err.Error())
		return
	}

	newState := organizationalUnitResourceModel{
		ID:                types.StringValue(ou.DistinguishedName),
		Path:              types.StringValue(ou.Path),
		DeleteSubtree:     plan.DeleteSubtree,
		DistinguishedName: types.StringValue(ou.DistinguishedName),
		Name:              types.StringValue(ou.Name),
		Description:       stringPointerToTerraform(ou.Description),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *organizationalUnitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state organizationalUnitResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteSubtree := false
	if !state.DeleteSubtree.IsNull() && !state.DeleteSubtree.IsUnknown() {
		deleteSubtree = state.DeleteSubtree.ValueBool()
	}

	if err := ad.DeleteOrganizationalUnit(ctx, r.client, state.DistinguishedName.ValueString(), deleteSubtree); err != nil {
		resp.Diagnostics.AddError("Unable to delete organizational unit", err.Error())
	}
}

func (r *organizationalUnitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func optionalString(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}

	result := value.ValueString()
	return &result
}

func stringPointerToTerraform(value *string) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(*value)
}
