package autoscaling

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
)

func ResourceLifecycleHook() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceLifecycleHookPut,
		ReadWithoutTimeout:   resourceLifecycleHookRead,
		UpdateWithoutTimeout: resourceLifecycleHookPut,
		DeleteWithoutTimeout: resourceLifecycleHookDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourceLifecycleHookImport,
		},

		Schema: map[string]*schema.Schema{
			"autoscaling_group_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"default_result": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"heartbeat_timeout": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"lifecycle_transition": {
				Type:     schema.TypeString,
				Required: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"notification_metadata": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"notification_target_arn": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"role_arn": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceLifecycleHookPut(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).AutoScalingConn()

	input := getPutLifecycleHookInput(d)
	name := d.Get("name").(string)

	log.Printf("[INFO] Putting Auto Scaling Lifecycle Hook: %s", input)
	_, err := tfresource.RetryWhenAWSErrMessageContains(ctx, 5*time.Minute,
		func() (interface{}, error) {
			return conn.PutLifecycleHookWithContext(ctx, input)
		},
		ErrCodeValidationError, "Unable to publish test message to notification target")

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "putting Auto Scaling Lifecycle Hook (%s): %s", name, err)
	}

	d.SetId(name)

	return append(diags, resourceLifecycleHookRead(ctx, d, meta)...)
}

func resourceLifecycleHookRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).AutoScalingConn()

	p, err := FindLifecycleHook(ctx, conn, d.Get("autoscaling_group_name").(string), d.Id())

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] Auto Scaling Lifecycle Hook %s not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading Auto Scaling Lifecycle Hook (%s): %s", d.Id(), err)
	}

	d.Set("default_result", p.DefaultResult)
	d.Set("heartbeat_timeout", p.HeartbeatTimeout)
	d.Set("lifecycle_transition", p.LifecycleTransition)
	d.Set("notification_metadata", p.NotificationMetadata)
	d.Set("notification_target_arn", p.NotificationTargetARN)
	d.Set("name", p.LifecycleHookName)
	d.Set("role_arn", p.RoleARN)

	return diags
}

func resourceLifecycleHookDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).AutoScalingConn()

	log.Printf("[INFO] Deleting Auto Scaling Lifecycle Hook: %s", d.Id())
	_, err := conn.DeleteLifecycleHookWithContext(ctx, &autoscaling.DeleteLifecycleHookInput{
		AutoScalingGroupName: aws.String(d.Get("autoscaling_group_name").(string)),
		LifecycleHookName:    aws.String(d.Id()),
	})

	if tfawserr.ErrMessageContains(err, ErrCodeValidationError, "not found") {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting Auto Scaling Lifecycle Hook (%s): %s", d.Id(), err)
	}

	return diags
}

func getPutLifecycleHookInput(d *schema.ResourceData) *autoscaling.PutLifecycleHookInput {
	var params = &autoscaling.PutLifecycleHookInput{
		AutoScalingGroupName: aws.String(d.Get("autoscaling_group_name").(string)),
		LifecycleHookName:    aws.String(d.Get("name").(string)),
	}

	if v, ok := d.GetOk("default_result"); ok {
		params.DefaultResult = aws.String(v.(string))
	}

	if v, ok := d.GetOk("heartbeat_timeout"); ok {
		params.HeartbeatTimeout = aws.Int64(int64(v.(int)))
	}

	if v, ok := d.GetOk("lifecycle_transition"); ok {
		params.LifecycleTransition = aws.String(v.(string))
	}

	if v, ok := d.GetOk("notification_metadata"); ok {
		params.NotificationMetadata = aws.String(v.(string))
	}

	if v, ok := d.GetOk("notification_target_arn"); ok {
		params.NotificationTargetARN = aws.String(v.(string))
	}

	if v, ok := d.GetOk("role_arn"); ok {
		params.RoleARN = aws.String(v.(string))
	}

	return params
}

func FindLifecycleHook(ctx context.Context, conn *autoscaling.AutoScaling, asgName, hookName string) (*autoscaling.LifecycleHook, error) {
	input := &autoscaling.DescribeLifecycleHooksInput{
		AutoScalingGroupName: aws.String(asgName),
		LifecycleHookNames:   aws.StringSlice([]string{hookName}),
	}

	output, err := conn.DescribeLifecycleHooksWithContext(ctx, input)

	if tfawserr.ErrMessageContains(err, ErrCodeValidationError, "not found") {
		return nil, &resource.NotFoundError{
			LastError:   err,
			LastRequest: input,
		}
	}

	if err != nil {
		return nil, err
	}

	if output == nil {
		return nil, tfresource.NewEmptyResultError(input)
	}

	for _, v := range output.LifecycleHooks {
		if aws.StringValue(v.LifecycleHookName) == hookName {
			return v, nil
		}
	}

	return nil, &resource.NotFoundError{LastRequest: input}
}

func resourceLifecycleHookImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	idParts := strings.SplitN(d.Id(), "/", 2)
	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		return nil, fmt.Errorf("unexpected format (%q), expected <asg-name>/<lifecycle-hook-name>", d.Id())
	}

	asgName := idParts[0]
	lifecycleHookName := idParts[1]

	d.Set("name", lifecycleHookName)
	d.Set("autoscaling_group_name", asgName)
	d.SetId(lifecycleHookName)

	return []*schema.ResourceData{d}, nil
}
