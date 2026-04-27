package main

import (
	"context"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: func() *schema.Provider {
			return &schema.Provider{
				Schema: map[string]*schema.Schema{
					"endpoint": {
						Type:        schema.TypeString,
						Optional:    true,
						DefaultFunc: schema.EnvDefaultFunc("AJAYJI_ENDPOINT", "http://localhost:14321"),
						Description: "The REST endpoint for the local Ajayji Daemon.",
					},
				},
				ResourcesMap: map[string]*schema.Resource{
					"ajayji_persona": resourceAjayjiPersona(),
					"ajayji_model":   resourceAjayjiModel(),
					"ajayji_huggingface_credential": resourceAjayjiHuggingFaceCredential(),
				},
				ConfigureContextFunc: providerConfigure,
			}
		},
	})
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	endpoint := d.Get("endpoint").(string)

	client := &AjayjiClient{
		Endpoint: endpoint,
		HttpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	return client, nil
}

func resourceAjayjiPersona() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePersonaCreate,
		ReadContext:   resourcePersonaRead,
		UpdateContext: resourcePersonaUpdate,
		DeleteContext: resourcePersonaDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"model": {
				Type:     schema.TypeString,
				Required: true,
			},
			"system_prompt": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"input_topic": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"output_topic": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"input_script": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"output_script": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourcePersonaCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)

	payload := PersonaPayload{
		Name:         d.Get("name").(string),
		Model:        d.Get("model").(string),
		SystemPrompt: d.Get("system_prompt").(string),
		InputTopic:   d.Get("input_topic").(string),
		OutputTopic:  d.Get("output_topic").(string),
		InputScript:  d.Get("input_script").(string),
		OutputScript: d.Get("output_script").(string),
	}

	created, err := client.CreatePersona(payload)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(created.ID)
	return resourcePersonaRead(ctx, d, m)
}

func resourcePersonaRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)
	id := d.Id()

	persona, err := client.GetPersona(id)
	if err != nil {
		return diag.FromErr(err)
	}

	// If the persona was deleted manually in the GUI (drift), tell Terraform it no longer exists
	if persona == nil {
		d.SetId("")
		return nil
	}

	d.Set("name", persona.Name)
	d.Set("model", persona.Model)
	d.Set("system_prompt", persona.SystemPrompt)
	d.Set("input_topic", persona.InputTopic)
	d.Set("output_topic", persona.OutputTopic)
	d.Set("input_script", persona.InputScript)
	d.Set("output_script", persona.OutputScript)

	return nil
}

func resourcePersonaUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)
	id := d.Id()

	payload := PersonaPayload{
		Name:         d.Get("name").(string),
		Model:        d.Get("model").(string),
		SystemPrompt: d.Get("system_prompt").(string),
		InputTopic:   d.Get("input_topic").(string),
		OutputTopic:  d.Get("output_topic").(string),
		InputScript:  d.Get("input_script").(string),
		OutputScript: d.Get("output_script").(string),
	}

	_, err := client.UpdatePersona(id, payload)
	if err != nil {
		return diag.FromErr(err)
	}

	return resourcePersonaRead(ctx, d, m)
}

func resourcePersonaDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)
	id := d.Id()

	err := client.DeletePersona(id)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

func resourceAjayjiModel() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceModelCreate,
		ReadContext:   resourceModelRead,
		DeleteContext: resourceModelDelete,
		// Models are immutable. If the filename changes, it forces a destroy and recreate.
		Schema: map[string]*schema.Schema{
			"repo": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true, // Cannot update in-place
			},
			"filename": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"huggingface_config_id": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true, // Changing the credential forces a new download
			},
		},
	}
}

func resourceModelCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)
	repo := d.Get("repo").(string)
	fileName := d.Get("filename").(string)
	configId := d.Get("huggingface_config_id").(string)

	// 1. Trigger the download
		err := client.PullModel(repo, fileName, configId)
	if err != nil {
		return diag.FromErr(err)
	}

		// 2. Poll the /models endpoint until the state is "downloaded" or "loaded"
	for {
		select {
		case <-ctx.Done():
			return diag.FromErr(ctx.Err())
		default:
			model, err := client.GetModel(fileName)
			if err != nil {
				return diag.FromErr(err)
			}
			
			if model != nil {
				// ADD THIS ERROR CHECK
				if model.State == "error" {
					return diag.Errorf("Model download failed inside Ajayji. Please check the Model Manager GUI.")
				}
				
				if model.State == "downloaded" || model.State == "loaded" {
					d.SetId(fileName) // Success!
					return resourceModelRead(ctx, d, m)
				}
			}
			// Wait 3 seconds before checking progress again
			time.Sleep(3 * time.Second)
		}
	}

}

func resourceModelRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)
	fileName := d.Id()

	model, err := client.GetModel(fileName)
	if err != nil {
		return diag.FromErr(err)
	}

	// If the model was manually deleted from the disk/GUI, clear the state for drift detection
	if model == nil {
		d.SetId("")
		return nil
	}

	d.Set("repo", model.Repo)
	d.Set("filename", model.FileName)
	return nil
}

func resourceModelDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)
	fileName := d.Id()

	err := client.DeleteModel(fileName)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

// --- Hugging Face Credential Resource ---

func resourceAjayjiHuggingFaceCredential() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceHFCredentialCreate,
		ReadContext:   resourceHFCredentialRead,
		UpdateContext: resourceHFCredentialUpdate,
		DeleteContext: resourceHFCredentialDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"token": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true, // Hides it from Terraform CLI output
			},
		},
	}
}

func resourceHFCredentialCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)

	payload := HuggingFaceConfigPayload{
		Name:  d.Get("name").(string),
		Token: d.Get("token").(string),
	}

	created, err := client.CreateHuggingFaceConfig(payload)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(created.ID)
	return resourceHFCredentialRead(ctx, d, m)
}

func resourceHFCredentialRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)
	id := d.Id()

	config, err := client.GetHuggingFaceConfig(id)
	if err != nil {
		return diag.FromErr(err)
	}

	if config == nil {
		d.SetId("") // Drift detection
		return nil
	}

	d.Set("name", config.Name)
	d.Set("token", config.Token)

	return nil
}

func resourceHFCredentialUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)
	id := d.Id()

	payload := HuggingFaceConfigPayload{
		Name:  d.Get("name").(string),
		Token: d.Get("token").(string),
	}

	_, err := client.UpdateHuggingFaceConfig(id, payload)
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceHFCredentialRead(ctx, d, m)
}

func resourceHFCredentialDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*AjayjiClient)
	id := d.Id()

	err := client.DeleteHuggingFaceConfig(id)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}