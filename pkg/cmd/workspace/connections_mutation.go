// Copyright The Orca Authors
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"encoding/json"
	"fmt"
	"strings"

	registry "github.com/orca-ae/orca-sdk-go"
	"github.com/spf13/cobra"
)

type connectionMutationMode string

const (
	connectionMutationModeCreate connectionMutationMode = "create"
	connectionMutationModeUpdate connectionMutationMode = "update"
)

const (
	flagConnectionName = "name"
	flagConnectionType = "type"
	flagOutput         = "output"

	flagPulsarServiceURL                    = "pulsar-service-url"
	flagPulsarAdminURL                      = "pulsar-admin-url"
	flagPulsarAuthType                      = "pulsar-auth-type"
	flagPulsarTokenSecretName               = "pulsar-token-secret-name"
	flagPulsarTokenSecretKey                = "pulsar-token-secret-key"
	flagPulsarOAuth2IssuerURL               = "pulsar-oauth2-issuer-url"
	flagPulsarOAuth2Audience                = "pulsar-oauth2-audience"
	flagPulsarOAuth2Scope                   = "pulsar-oauth2-scope"
	flagPulsarOAuth2SecretName              = "pulsar-oauth2-secret-name"
	flagPulsarOAuth2SecretKey               = "pulsar-oauth2-secret-key"
	flagPulsarGenericAuthPlugin             = "pulsar-generic-auth-plugin"
	flagPulsarGenericAuthParams             = "pulsar-generic-auth-params"
	flagPulsarTLSEnabled                    = "pulsar-tls-enabled"
	flagPulsarTLSAllowInsecure              = "pulsar-tls-allow-insecure"
	flagPulsarTLSEnableHostnameVerification = "pulsar-tls-enable-hostname-verification"
	flagPulsarTLSTrustSecretName            = "pulsar-tls-trust-secret-name"
	flagPulsarTLSTrustSecretKey             = "pulsar-tls-trust-secret-key"
	flagPulsarTLSCertSecretName             = "pulsar-tls-cert-secret-name"
	flagPulsarTLSCertSecretKey              = "pulsar-tls-cert-secret-key"
	flagPulsarTLSKeySecretName              = "pulsar-tls-key-secret-name"
	flagPulsarTLSKeySecretKey               = "pulsar-tls-key-secret-key"
	flagKafkaBootstrapServers               = "kafka-bootstrap-servers"
	flagKafkaAuthType                       = "kafka-auth-type"
	flagKafkaPlainSecretName                = "kafka-plain-secret-name"
	flagKafkaPlainUsernameKey               = "kafka-plain-username-key"
	flagKafkaPlainPasswordKey               = "kafka-plain-password-key"
	flagKafkaScramSecretName                = "kafka-scram-secret-name"
	flagKafkaScramUsernameKey               = "kafka-scram-username-key"
	flagKafkaScramPasswordKey               = "kafka-scram-password-key"
	flagKafkaScramHash                      = "kafka-scram-hash"
	flagKafkaOAuth2IssuerURL                = "kafka-oauth2-issuer-url"
	flagKafkaOAuth2Audience                 = "kafka-oauth2-audience"
	flagKafkaOAuth2Scope                    = "kafka-oauth2-scope"
	flagKafkaOAuth2SecretName               = "kafka-oauth2-secret-name"
	flagKafkaOAuth2SecretKey                = "kafka-oauth2-secret-key"
	flagKafkaGenericAuthPlugin              = "kafka-generic-auth-plugin"
	flagKafkaGenericAuthParams              = "kafka-generic-auth-params"
	flagKafkaTLSEnabled                     = "kafka-tls-enabled"
	flagKafkaTLSTrustSecretName             = "kafka-tls-trust-secret-name"
	flagKafkaTLSTrustFileKey                = "kafka-tls-trust-file-key"
	flagKafkaTLSTrustPasswordKey            = "kafka-tls-trust-password-key"
	flagKafkaTLSTrustType                   = "kafka-tls-trust-type"
	flagKafkaTLSKeySecretName               = "kafka-tls-key-secret-name"
	flagKafkaTLSKeyFileKey                  = "kafka-tls-key-file-key"
	flagKafkaTLSKeyPasswordKey              = "kafka-tls-key-password-key"
	flagKafkaTLSKeyKeyPasswordKey           = "kafka-tls-key-key-password-key"
	flagKafkaTLSKeyType                     = "kafka-tls-key-type"
	flagOtherEndpoint                       = "other-endpoint"
	flagOtherProperty                       = "other-property"
	flagOtherSecretName                     = "other-secret-name"
	flagOtherSecretKey                      = "other-secret-key"
)

const (
	authTypeNone    = "none"
	authTypeToken   = "token"
	authTypeOAuth2  = "oauth2"
	authTypeGeneric = "generic"

	kafkaAuthTypePlain = "plain"
	kafkaAuthTypeScram = "scram"
)

var (
	pulsarFlagNames = []string{
		flagPulsarServiceURL,
		flagPulsarAdminURL,
		flagPulsarAuthType,
		flagPulsarTokenSecretName,
		flagPulsarTokenSecretKey,
		flagPulsarOAuth2IssuerURL,
		flagPulsarOAuth2Audience,
		flagPulsarOAuth2Scope,
		flagPulsarOAuth2SecretName,
		flagPulsarOAuth2SecretKey,
		flagPulsarGenericAuthPlugin,
		flagPulsarGenericAuthParams,
		flagPulsarTLSEnabled,
		flagPulsarTLSAllowInsecure,
		flagPulsarTLSEnableHostnameVerification,
		flagPulsarTLSTrustSecretName,
		flagPulsarTLSTrustSecretKey,
		flagPulsarTLSCertSecretName,
		flagPulsarTLSCertSecretKey,
		flagPulsarTLSKeySecretName,
		flagPulsarTLSKeySecretKey,
	}
	pulsarTokenFlagNames = []string{
		flagPulsarTokenSecretName,
		flagPulsarTokenSecretKey,
	}
	pulsarOAuth2FlagNames = []string{
		flagPulsarOAuth2IssuerURL,
		flagPulsarOAuth2Audience,
		flagPulsarOAuth2Scope,
		flagPulsarOAuth2SecretName,
		flagPulsarOAuth2SecretKey,
	}
	pulsarGenericAuthFlagNames = []string{
		flagPulsarGenericAuthPlugin,
		flagPulsarGenericAuthParams,
	}
	pulsarTLSFlagNames = []string{
		flagPulsarTLSEnabled,
		flagPulsarTLSAllowInsecure,
		flagPulsarTLSEnableHostnameVerification,
		flagPulsarTLSTrustSecretName,
		flagPulsarTLSTrustSecretKey,
		flagPulsarTLSCertSecretName,
		flagPulsarTLSCertSecretKey,
		flagPulsarTLSKeySecretName,
		flagPulsarTLSKeySecretKey,
	}
	pulsarTLSSubFlagNames = []string{
		flagPulsarTLSAllowInsecure,
		flagPulsarTLSEnableHostnameVerification,
		flagPulsarTLSTrustSecretName,
		flagPulsarTLSTrustSecretKey,
		flagPulsarTLSCertSecretName,
		flagPulsarTLSCertSecretKey,
		flagPulsarTLSKeySecretName,
		flagPulsarTLSKeySecretKey,
	}
	kafkaFlagNames = []string{
		flagKafkaBootstrapServers,
		flagKafkaAuthType,
		flagKafkaPlainSecretName,
		flagKafkaPlainUsernameKey,
		flagKafkaPlainPasswordKey,
		flagKafkaScramSecretName,
		flagKafkaScramUsernameKey,
		flagKafkaScramPasswordKey,
		flagKafkaScramHash,
		flagKafkaOAuth2IssuerURL,
		flagKafkaOAuth2Audience,
		flagKafkaOAuth2Scope,
		flagKafkaOAuth2SecretName,
		flagKafkaOAuth2SecretKey,
		flagKafkaGenericAuthPlugin,
		flagKafkaGenericAuthParams,
		flagKafkaTLSEnabled,
		flagKafkaTLSTrustSecretName,
		flagKafkaTLSTrustFileKey,
		flagKafkaTLSTrustPasswordKey,
		flagKafkaTLSTrustType,
		flagKafkaTLSKeySecretName,
		flagKafkaTLSKeyFileKey,
		flagKafkaTLSKeyPasswordKey,
		flagKafkaTLSKeyKeyPasswordKey,
		flagKafkaTLSKeyType,
	}
	kafkaPlainFlagNames = []string{
		flagKafkaPlainSecretName,
		flagKafkaPlainUsernameKey,
		flagKafkaPlainPasswordKey,
	}
	kafkaScramFlagNames = []string{
		flagKafkaScramSecretName,
		flagKafkaScramUsernameKey,
		flagKafkaScramPasswordKey,
		flagKafkaScramHash,
	}
	kafkaOAuth2FlagNames = []string{
		flagKafkaOAuth2IssuerURL,
		flagKafkaOAuth2Audience,
		flagKafkaOAuth2Scope,
		flagKafkaOAuth2SecretName,
		flagKafkaOAuth2SecretKey,
	}
	kafkaGenericAuthFlagNames = []string{
		flagKafkaGenericAuthPlugin,
		flagKafkaGenericAuthParams,
	}
	kafkaTLSFlagNames = []string{
		flagKafkaTLSEnabled,
		flagKafkaTLSTrustSecretName,
		flagKafkaTLSTrustFileKey,
		flagKafkaTLSTrustPasswordKey,
		flagKafkaTLSTrustType,
		flagKafkaTLSKeySecretName,
		flagKafkaTLSKeyFileKey,
		flagKafkaTLSKeyPasswordKey,
		flagKafkaTLSKeyKeyPasswordKey,
		flagKafkaTLSKeyType,
	}
	kafkaTLSSubFlagNames = []string{
		flagKafkaTLSTrustSecretName,
		flagKafkaTLSTrustFileKey,
		flagKafkaTLSTrustPasswordKey,
		flagKafkaTLSTrustType,
		flagKafkaTLSKeySecretName,
		flagKafkaTLSKeyFileKey,
		flagKafkaTLSKeyPasswordKey,
		flagKafkaTLSKeyKeyPasswordKey,
		flagKafkaTLSKeyType,
	}
	otherFlagNames = []string{
		flagOtherEndpoint,
		flagOtherProperty,
		flagOtherSecretName,
		flagOtherSecretKey,
	}
)

type connectionMutationOptions struct {
	mode   connectionMutationMode
	output string
	name   string
	typ    string

	pulsarServiceURL                    string
	pulsarAdminURL                      string
	pulsarAuthType                      string
	pulsarTokenSecretName               string
	pulsarTokenSecretKey                string
	pulsarOAuth2IssuerURL               string
	pulsarOAuth2Audience                string
	pulsarOAuth2Scope                   string
	pulsarOAuth2SecretName              string
	pulsarOAuth2SecretKey               string
	pulsarGenericAuthPlugin             string
	pulsarGenericAuthParams             string
	pulsarTLSEnabled                    bool
	pulsarTLSAllowInsecure              bool
	pulsarTLSEnableHostnameVerification bool
	pulsarTLSTrustSecretName            string
	pulsarTLSTrustSecretKey             string
	pulsarTLSCertSecretName             string
	pulsarTLSCertSecretKey              string
	pulsarTLSKeySecretName              string
	pulsarTLSKeySecretKey               string

	kafkaBootstrapServers     string
	kafkaAuthType             string
	kafkaPlainSecretName      string
	kafkaPlainUsernameKey     string
	kafkaPlainPasswordKey     string
	kafkaScramSecretName      string
	kafkaScramUsernameKey     string
	kafkaScramPasswordKey     string
	kafkaScramHash            string
	kafkaOAuth2IssuerURL      string
	kafkaOAuth2Audience       string
	kafkaOAuth2Scope          string
	kafkaOAuth2SecretName     string
	kafkaOAuth2SecretKey      string
	kafkaGenericAuthPlugin    string
	kafkaGenericAuthParams    string
	kafkaTLSEnabled           bool
	kafkaTLSTrustSecretName   string
	kafkaTLSTrustFileKey      string
	kafkaTLSTrustPasswordKey  string
	kafkaTLSTrustType         string
	kafkaTLSKeySecretName     string
	kafkaTLSKeyFileKey        string
	kafkaTLSKeyPasswordKey    string
	kafkaTLSKeyKeyPasswordKey string
	kafkaTLSKeyType           string

	otherEndpoint   string
	otherProperties []string
	otherSecretName string
	otherSecretKey  string
}

type pulsarOAuth2Config struct {
	Audience      string `json:"audience,omitempty"`
	IssuerURL     string `json:"issuerUrl,omitempty"`
	KeySecretKey  string `json:"keySecretKey,omitempty"`
	KeySecretName string `json:"keySecretName,omitempty"`
	Scope         string `json:"scope,omitempty"`
}

type genericAuthConfig struct {
	ClientAuthenticationParameters string `json:"clientAuthenticationParameters,omitempty"`
	ClientAuthenticationPlugin     string `json:"clientAuthenticationPlugin,omitempty"`
}

type kafkaAuthenticationConfig struct {
	GenericAuth     *genericAuthConfig    `json:"genericAuth,omitempty"`
	OAuth2Config    *pulsarOAuth2Config   `json:"oauth2Config,omitempty"`
	PlainAuthConfig *kafkaPlainAuthConfig `json:"plainAuthConfig,omitempty"`
	ScramAuthConfig *kafkaScramAuthConfig `json:"scramAuthConfig,omitempty"`
}

type kafkaPlainAuthConfig struct {
	PasswordKey string `json:"passwordKey,omitempty"`
	SecretName  string `json:"secretName,omitempty"`
	UsernameKey string `json:"usernameKey,omitempty"`
}

type kafkaScramAuthConfig struct {
	HashAlgorithm string `json:"hashAlgorithm,omitempty"`
	PasswordKey   string `json:"passwordKey,omitempty"`
	SecretName    string `json:"secretName,omitempty"`
	UsernameKey   string `json:"usernameKey,omitempty"`
}

type kafkaTLSConfig struct {
	Enabled          bool                   `json:"enabled,omitempty"`
	KeyStoreConfig   *kafkaKeyStoreConfig   `json:"keyStoreConfig,omitempty"`
	TrustStoreConfig *kafkaTrustStoreConfig `json:"trustStoreConfig,omitempty"`
}

type kafkaTrustStoreConfig struct {
	FileKey     string `json:"fileKey,omitempty"`
	PasswordKey string `json:"passwordKey,omitempty"`
	SecretName  string `json:"secretName,omitempty"`
	Type        string `json:"type,omitempty"`
}

type kafkaKeyStoreConfig struct {
	FileKey        string `json:"fileKey,omitempty"`
	KeyPasswordKey string `json:"keyPasswordKey,omitempty"`
	PasswordKey    string `json:"passwordKey,omitempty"`
	SecretName     string `json:"secretName,omitempty"`
	Type           string `json:"type,omitempty"`
}

func newConnectionMutationOptions(mode connectionMutationMode) *connectionMutationOptions {
	return &connectionMutationOptions{
		mode:   mode,
		output: "text",
	}
}

func (o *connectionMutationOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.name, flagConnectionName, "", "Workspace connection name")
	cmd.Flags().StringVar(&o.typ, flagConnectionType, "", "Connection type (pulsar, kafka, other)")
	cmd.Flags().StringVarP(&o.output, flagOutput, "o", o.output, "Output format (text, json, yaml)")
	_ = cmd.MarkFlagRequired(flagConnectionName)
	if o.mode == connectionMutationModeCreate {
		_ = cmd.MarkFlagRequired(flagConnectionType)
	}

	cmd.Flags().StringVar(&o.pulsarServiceURL, flagPulsarServiceURL, "", "Pulsar broker service URL")
	cmd.Flags().StringVar(&o.pulsarAdminURL, flagPulsarAdminURL, "", "Pulsar admin URL")
	cmd.Flags().StringVar(&o.pulsarAuthType, flagPulsarAuthType, "", "Pulsar auth type (none, token, oauth2, generic)")
	cmd.Flags().StringVar(&o.pulsarTokenSecretName, flagPulsarTokenSecretName, "", "Kubernetes Secret name for the Pulsar token")
	cmd.Flags().StringVar(&o.pulsarTokenSecretKey, flagPulsarTokenSecretKey, "", "Secret key for the Pulsar token")
	cmd.Flags().StringVar(&o.pulsarOAuth2IssuerURL, flagPulsarOAuth2IssuerURL, "", "Pulsar OAuth2 issuer URL")
	cmd.Flags().StringVar(&o.pulsarOAuth2Audience, flagPulsarOAuth2Audience, "", "Pulsar OAuth2 audience")
	cmd.Flags().StringVar(&o.pulsarOAuth2Scope, flagPulsarOAuth2Scope, "", "Pulsar OAuth2 scope")
	cmd.Flags().StringVar(&o.pulsarOAuth2SecretName, flagPulsarOAuth2SecretName, "", "Kubernetes Secret name for Pulsar OAuth2 credentials")
	cmd.Flags().StringVar(&o.pulsarOAuth2SecretKey, flagPulsarOAuth2SecretKey, "", "Secret key for Pulsar OAuth2 credentials")
	cmd.Flags().StringVar(&o.pulsarGenericAuthPlugin, flagPulsarGenericAuthPlugin, "", "Pulsar generic auth plugin")
	cmd.Flags().StringVar(&o.pulsarGenericAuthParams, flagPulsarGenericAuthParams, "", "Pulsar generic auth parameters")
	cmd.Flags().BoolVar(&o.pulsarTLSEnabled, flagPulsarTLSEnabled, false, "Enable TLS for the Pulsar connection")
	cmd.Flags().BoolVar(&o.pulsarTLSAllowInsecure, flagPulsarTLSAllowInsecure, false, "Allow insecure TLS for the Pulsar connection")
	cmd.Flags().BoolVar(&o.pulsarTLSEnableHostnameVerification, flagPulsarTLSEnableHostnameVerification, false, "Enable TLS hostname verification for the Pulsar connection")
	cmd.Flags().StringVar(&o.pulsarTLSTrustSecretName, flagPulsarTLSTrustSecretName, "", "Kubernetes Secret name for Pulsar TLS trust certs")
	cmd.Flags().StringVar(&o.pulsarTLSTrustSecretKey, flagPulsarTLSTrustSecretKey, "", "Secret key for Pulsar TLS trust certs")
	cmd.Flags().StringVar(&o.pulsarTLSCertSecretName, flagPulsarTLSCertSecretName, "", "Kubernetes Secret name for the Pulsar TLS client certificate")
	cmd.Flags().StringVar(&o.pulsarTLSCertSecretKey, flagPulsarTLSCertSecretKey, "", "Secret key for the Pulsar TLS client certificate")
	cmd.Flags().StringVar(&o.pulsarTLSKeySecretName, flagPulsarTLSKeySecretName, "", "Kubernetes Secret name for the Pulsar TLS client key")
	cmd.Flags().StringVar(&o.pulsarTLSKeySecretKey, flagPulsarTLSKeySecretKey, "", "Secret key for the Pulsar TLS client key")

	cmd.Flags().StringVar(&o.kafkaBootstrapServers, flagKafkaBootstrapServers, "", "Kafka bootstrap servers")
	cmd.Flags().StringVar(&o.kafkaAuthType, flagKafkaAuthType, "", "Kafka auth type (none, plain, scram, oauth2, generic)")
	cmd.Flags().StringVar(&o.kafkaPlainSecretName, flagKafkaPlainSecretName, "", "Kubernetes Secret name for Kafka plain auth")
	cmd.Flags().StringVar(&o.kafkaPlainUsernameKey, flagKafkaPlainUsernameKey, "", "Secret key for Kafka plain auth username")
	cmd.Flags().StringVar(&o.kafkaPlainPasswordKey, flagKafkaPlainPasswordKey, "", "Secret key for Kafka plain auth password")
	cmd.Flags().StringVar(&o.kafkaScramSecretName, flagKafkaScramSecretName, "", "Kubernetes Secret name for Kafka SCRAM auth")
	cmd.Flags().StringVar(&o.kafkaScramUsernameKey, flagKafkaScramUsernameKey, "", "Secret key for Kafka SCRAM username")
	cmd.Flags().StringVar(&o.kafkaScramPasswordKey, flagKafkaScramPasswordKey, "", "Secret key for Kafka SCRAM password")
	cmd.Flags().StringVar(&o.kafkaScramHash, flagKafkaScramHash, "", "Kafka SCRAM hash algorithm (sha-256, sha-512)")
	cmd.Flags().StringVar(&o.kafkaOAuth2IssuerURL, flagKafkaOAuth2IssuerURL, "", "Kafka OAuth2 issuer URL")
	cmd.Flags().StringVar(&o.kafkaOAuth2Audience, flagKafkaOAuth2Audience, "", "Kafka OAuth2 audience")
	cmd.Flags().StringVar(&o.kafkaOAuth2Scope, flagKafkaOAuth2Scope, "", "Kafka OAuth2 scope")
	cmd.Flags().StringVar(&o.kafkaOAuth2SecretName, flagKafkaOAuth2SecretName, "", "Kubernetes Secret name for Kafka OAuth2 credentials")
	cmd.Flags().StringVar(&o.kafkaOAuth2SecretKey, flagKafkaOAuth2SecretKey, "", "Secret key for Kafka OAuth2 credentials")
	cmd.Flags().StringVar(&o.kafkaGenericAuthPlugin, flagKafkaGenericAuthPlugin, "", "Kafka generic auth plugin")
	cmd.Flags().StringVar(&o.kafkaGenericAuthParams, flagKafkaGenericAuthParams, "", "Kafka generic auth parameters")
	cmd.Flags().BoolVar(&o.kafkaTLSEnabled, flagKafkaTLSEnabled, false, "Enable TLS for the Kafka connection")
	cmd.Flags().StringVar(&o.kafkaTLSTrustSecretName, flagKafkaTLSTrustSecretName, "", "Kubernetes Secret name for the Kafka trust store")
	cmd.Flags().StringVar(&o.kafkaTLSTrustFileKey, flagKafkaTLSTrustFileKey, "", "Secret key containing the Kafka trust store file")
	cmd.Flags().StringVar(&o.kafkaTLSTrustPasswordKey, flagKafkaTLSTrustPasswordKey, "", "Secret key containing the Kafka trust store password")
	cmd.Flags().StringVar(&o.kafkaTLSTrustType, flagKafkaTLSTrustType, "", "Kafka trust store type (JKS, PEM, PKCS12)")
	cmd.Flags().StringVar(&o.kafkaTLSKeySecretName, flagKafkaTLSKeySecretName, "", "Kubernetes Secret name for the Kafka key store")
	cmd.Flags().StringVar(&o.kafkaTLSKeyFileKey, flagKafkaTLSKeyFileKey, "", "Secret key containing the Kafka key store file")
	cmd.Flags().StringVar(&o.kafkaTLSKeyPasswordKey, flagKafkaTLSKeyPasswordKey, "", "Secret key containing the Kafka key store password")
	cmd.Flags().StringVar(&o.kafkaTLSKeyKeyPasswordKey, flagKafkaTLSKeyKeyPasswordKey, "", "Secret key containing the Kafka private key password")
	cmd.Flags().StringVar(&o.kafkaTLSKeyType, flagKafkaTLSKeyType, "", "Kafka key store type (JKS, PEM, PKCS12)")

	cmd.Flags().StringVar(&o.otherEndpoint, flagOtherEndpoint, "", "Endpoint for the generic connection")
	cmd.Flags().StringArrayVar(&o.otherProperties, flagOtherProperty, nil, "Generic connection property in key=value format (repeatable)")
	cmd.Flags().StringVar(&o.otherSecretName, flagOtherSecretName, "", "Kubernetes Secret name for the generic connection secret")
	cmd.Flags().StringVar(&o.otherSecretKey, flagOtherSecretKey, "", "Secret key for the generic connection secret")
}

func (o *connectionMutationOptions) buildCreatePayload(cmd *cobra.Command) (registry.ConnectionConfig, error) {
	if err := o.validateCommon(cmd); err != nil {
		return registry.ConnectionConfig{}, err
	}

	connectionType, err := parseConnectionType(o.typ)
	if err != nil {
		return registry.ConnectionConfig{}, err
	}
	if err := o.validateTypeFlagUsage(cmd, connectionType); err != nil {
		return registry.ConnectionConfig{}, err
	}

	spec, err := o.buildSpec(cmd, connectionType, nil)
	if err != nil {
		return registry.ConnectionConfig{}, err
	}

	return registry.ConnectionConfig{
		Name: o.name,
		Spec: spec,
	}, nil
}

func (o *connectionMutationOptions) buildUpdatedPayload(cmd *cobra.Command, existing registry.ConnectionConfig) (registry.ConnectionConfig, error) {
	targetType := existing.Spec.Type
	if o.flagChanged(cmd, flagConnectionType) {
		parsedType, err := parseConnectionType(o.typ)
		if err != nil {
			return registry.ConnectionConfig{}, err
		}
		targetType = parsedType
	}
	if err := o.validateTypeFlagUsage(cmd, targetType); err != nil {
		return registry.ConnectionConfig{}, err
	}

	var existingSpec *registry.ConnectionSpec
	if !o.flagChanged(cmd, flagConnectionType) || targetType == existing.Spec.Type {
		existingSpec = &existing.Spec
	}

	spec, err := o.buildSpec(cmd, targetType, existingSpec)
	if err != nil {
		return registry.ConnectionConfig{}, err
	}

	return registry.ConnectionConfig{
		Name: o.name,
		Spec: spec,
	}, nil
}

func (o *connectionMutationOptions) validateCommon(cmd *cobra.Command) error {
	if err := validateConnectionsOutput(o.output); err != nil {
		return err
	}
	if err := validateWorkspaceName("connection", o.name); err != nil {
		return err
	}
	if o.mode == connectionMutationModeCreate && !o.flagChanged(cmd, flagConnectionType) {
		return fmt.Errorf("--%s is required", flagConnectionType)
	}
	return nil
}

func (o *connectionMutationOptions) validateTypeFlagUsage(cmd *cobra.Command, connectionType registry.ConnectionType) error {
	switch connectionType {
	case registry.ConnectionTypePulsar:
		if o.changedAny(cmd, kafkaFlagNames...) {
			return fmt.Errorf("Kafka flags cannot be used when --type=%s", connectionType)
		}
		if o.changedAny(cmd, otherFlagNames...) {
			return fmt.Errorf("Other connection flags cannot be used when --type=%s", connectionType)
		}
	case registry.ConnectionTypeKafka:
		if o.changedAny(cmd, pulsarFlagNames...) {
			return fmt.Errorf("Pulsar flags cannot be used when --type=%s", connectionType)
		}
		if o.changedAny(cmd, otherFlagNames...) {
			return fmt.Errorf("Other connection flags cannot be used when --type=%s", connectionType)
		}
	case registry.ConnectionTypeOther:
		if o.changedAny(cmd, pulsarFlagNames...) {
			return fmt.Errorf("Pulsar flags cannot be used when --type=%s", connectionType)
		}
		if o.changedAny(cmd, kafkaFlagNames...) {
			return fmt.Errorf("Kafka flags cannot be used when --type=%s", connectionType)
		}
	default:
		return fmt.Errorf("unsupported connection type %q", connectionType)
	}
	return nil
}

func (o *connectionMutationOptions) buildSpec(cmd *cobra.Command, connectionType registry.ConnectionType, existing *registry.ConnectionSpec) (registry.ConnectionSpec, error) {
	spec := registry.ConnectionSpec{Type: connectionType}

	switch connectionType {
	case registry.ConnectionTypePulsar:
		var current *registry.PulsarConnectionConfig
		if existing != nil {
			current = existing.Pulsar
		}
		cfg, err := o.buildPulsarConfig(cmd, current)
		if err != nil {
			return registry.ConnectionSpec{}, err
		}
		spec.Pulsar = cfg
	case registry.ConnectionTypeKafka:
		var current *registry.KafkaConnectionConfig
		if existing != nil {
			current = existing.Kafka
		}
		cfg, err := o.buildKafkaConfig(cmd, current)
		if err != nil {
			return registry.ConnectionSpec{}, err
		}
		spec.Kafka = cfg
	case registry.ConnectionTypeOther:
		var current *registry.OtherConnectionConfig
		if existing != nil {
			current = existing.Other
		}
		cfg, err := o.buildOtherConfig(cmd, current)
		if err != nil {
			return registry.ConnectionSpec{}, err
		}
		spec.Other = cfg
	default:
		return registry.ConnectionSpec{}, fmt.Errorf("unsupported connection type %q", connectionType)
	}

	if err := validateBuiltConnectionSpec(spec); err != nil {
		return registry.ConnectionSpec{}, err
	}
	return spec, nil
}

func (o *connectionMutationOptions) buildPulsarConfig(cmd *cobra.Command, existing *registry.PulsarConnectionConfig) (*registry.PulsarConnectionConfig, error) {
	cfg := clonePulsarConnectionConfig(existing)
	if cfg == nil {
		cfg = &registry.PulsarConnectionConfig{}
	}

	if o.flagChanged(cmd, flagPulsarServiceURL) {
		cfg.ServiceURL = strings.TrimSpace(o.pulsarServiceURL)
	}
	if o.flagChanged(cmd, flagPulsarAdminURL) {
		cfg.AdminURL = strings.TrimSpace(o.pulsarAdminURL)
	}

	auth, err := o.buildPulsarAuth(cmd, cfg.Authentication)
	if err != nil {
		return nil, err
	}
	cfg.Authentication = auth

	tls, err := o.buildPulsarTLS(cmd, cfg.TLS)
	if err != nil {
		return nil, err
	}
	cfg.TLS = tls

	return cfg, nil
}

func (o *connectionMutationOptions) buildKafkaConfig(cmd *cobra.Command, existing *registry.KafkaConnectionConfig) (*registry.KafkaConnectionConfig, error) {
	cfg := cloneKafkaConnectionConfig(existing)
	if cfg == nil {
		cfg = &registry.KafkaConnectionConfig{}
	}

	if o.flagChanged(cmd, flagKafkaBootstrapServers) {
		cfg.BootstrapServers = strings.TrimSpace(o.kafkaBootstrapServers)
	}

	auth, err := o.buildKafkaAuth(cmd, cfg.Authentication)
	if err != nil {
		return nil, err
	}
	cfg.Authentication = auth

	tls, err := o.buildKafkaTLS(cmd, cfg.TLS)
	if err != nil {
		return nil, err
	}
	cfg.TLS = tls

	return cfg, nil
}

func (o *connectionMutationOptions) buildOtherConfig(cmd *cobra.Command, existing *registry.OtherConnectionConfig) (*registry.OtherConnectionConfig, error) {
	cfg := cloneOtherConnectionConfig(existing)
	if cfg == nil {
		cfg = &registry.OtherConnectionConfig{}
	}

	if o.flagChanged(cmd, flagOtherEndpoint) {
		cfg.Endpoint = strings.TrimSpace(o.otherEndpoint)
	}

	if o.flagChanged(cmd, flagOtherProperty) {
		properties, err := parseOtherProperties(o.otherProperties)
		if err != nil {
			return nil, err
		}
		cfg.Properties = mergeStringMap(cfg.Properties, properties)
	}

	if o.flagChanged(cmd, flagOtherSecretName) || o.flagChanged(cmd, flagOtherSecretKey) {
		ref := cloneSecretKeyRef(cfg.SecretRef)
		if ref == nil {
			ref = &registry.SecretKeyRef{}
		}
		if o.flagChanged(cmd, flagOtherSecretName) {
			ref.Name = strings.TrimSpace(o.otherSecretName)
		}
		if o.flagChanged(cmd, flagOtherSecretKey) {
			ref.Key = strings.TrimSpace(o.otherSecretKey)
		}
		if err := validateSecretKeyRef(ref, "other secret"); err != nil {
			return nil, err
		}
		cfg.SecretRef = ref
	}

	return cfg, nil
}

func (o *connectionMutationOptions) buildPulsarAuth(cmd *cobra.Command, existing *registry.PulsarAuthConfig) (*registry.PulsarAuthConfig, error) {
	existingMode := detectPulsarAuthMode(existing)
	targetMode, err := o.resolvePulsarAuthMode(cmd, existingMode)
	if err != nil {
		return nil, err
	}
	if targetMode == authTypeNone {
		return nil, nil
	}

	switch targetMode {
	case authTypeToken:
		ref := &registry.SecretKeyRef{}
		if existingMode == authTypeToken && existing != nil && existing.Token != nil {
			ref = cloneSecretKeyRef(existing.Token)
		}
		if o.flagChanged(cmd, flagPulsarTokenSecretName) {
			ref.Name = strings.TrimSpace(o.pulsarTokenSecretName)
		}
		if o.flagChanged(cmd, flagPulsarTokenSecretKey) {
			ref.Key = strings.TrimSpace(o.pulsarTokenSecretKey)
		}
		if err := validateSecretKeyRef(ref, "Pulsar token"); err != nil {
			return nil, err
		}
		return &registry.PulsarAuthConfig{Token: ref}, nil
	case authTypeOAuth2:
		cfg := pulsarOAuth2Config{}
		if existingMode == authTypeOAuth2 && existing != nil {
			cfg, err = decodeMapObject[pulsarOAuth2Config](existing.OAuth2)
			if err != nil {
				return nil, fmt.Errorf("failed to decode existing Pulsar OAuth2 config: %w", err)
			}
		}
		if o.flagChanged(cmd, flagPulsarOAuth2IssuerURL) {
			cfg.IssuerURL = strings.TrimSpace(o.pulsarOAuth2IssuerURL)
		}
		if o.flagChanged(cmd, flagPulsarOAuth2Audience) {
			cfg.Audience = strings.TrimSpace(o.pulsarOAuth2Audience)
		}
		if o.flagChanged(cmd, flagPulsarOAuth2Scope) {
			cfg.Scope = strings.TrimSpace(o.pulsarOAuth2Scope)
		}
		if o.flagChanged(cmd, flagPulsarOAuth2SecretName) {
			cfg.KeySecretName = strings.TrimSpace(o.pulsarOAuth2SecretName)
		}
		if o.flagChanged(cmd, flagPulsarOAuth2SecretKey) {
			cfg.KeySecretKey = strings.TrimSpace(o.pulsarOAuth2SecretKey)
		}
		if strings.TrimSpace(cfg.KeySecretName) == "" || strings.TrimSpace(cfg.KeySecretKey) == "" {
			return nil, fmt.Errorf("Pulsar OAuth2 requires both --%s and --%s", flagPulsarOAuth2SecretName, flagPulsarOAuth2SecretKey)
		}
		encoded, err := encodeMapObject(cfg)
		if err != nil {
			return nil, err
		}
		return &registry.PulsarAuthConfig{OAuth2: encoded}, nil
	case authTypeGeneric:
		cfg := genericAuthConfig{}
		if existingMode == authTypeGeneric && existing != nil {
			cfg, err = decodeMapObject[genericAuthConfig](existing.GenericAuth)
			if err != nil {
				return nil, fmt.Errorf("failed to decode existing Pulsar generic auth config: %w", err)
			}
		}
		if o.flagChanged(cmd, flagPulsarGenericAuthPlugin) {
			cfg.ClientAuthenticationPlugin = strings.TrimSpace(o.pulsarGenericAuthPlugin)
		}
		if o.flagChanged(cmd, flagPulsarGenericAuthParams) {
			cfg.ClientAuthenticationParameters = strings.TrimSpace(o.pulsarGenericAuthParams)
		}
		if strings.TrimSpace(cfg.ClientAuthenticationPlugin) == "" || strings.TrimSpace(cfg.ClientAuthenticationParameters) == "" {
			return nil, fmt.Errorf("Pulsar generic auth requires both --%s and --%s", flagPulsarGenericAuthPlugin, flagPulsarGenericAuthParams)
		}
		encoded, err := encodeMapObject(cfg)
		if err != nil {
			return nil, err
		}
		return &registry.PulsarAuthConfig{GenericAuth: encoded}, nil
	default:
		return existing, nil
	}
}

func (o *connectionMutationOptions) buildPulsarTLS(cmd *cobra.Command, existing *registry.PulsarTLSConfig) (*registry.PulsarTLSConfig, error) {
	if !o.changedAny(cmd, pulsarTLSFlagNames...) {
		return clonePulsarTLSConfig(existing), nil
	}
	if o.flagChanged(cmd, flagPulsarTLSEnabled) && !o.pulsarTLSEnabled {
		if o.changedAny(cmd, pulsarTLSSubFlagNames...) {
			return nil, fmt.Errorf("Pulsar TLS sub-flags cannot be used with --%s=false", flagPulsarTLSEnabled)
		}
		return nil, nil
	}

	cfg := clonePulsarTLSConfig(existing)
	if cfg == nil {
		cfg = &registry.PulsarTLSConfig{}
	}
	if o.flagChanged(cmd, flagPulsarTLSEnabled) {
		cfg.Enabled = o.pulsarTLSEnabled
	}
	if !o.flagChanged(cmd, flagPulsarTLSEnabled) && o.changedAny(cmd, pulsarTLSSubFlagNames...) {
		cfg.Enabled = true
	}
	if o.flagChanged(cmd, flagPulsarTLSAllowInsecure) {
		cfg.AllowInsecureConnection = o.pulsarTLSAllowInsecure
	}
	if o.flagChanged(cmd, flagPulsarTLSEnableHostnameVerification) {
		cfg.EnableHostnameVerification = boolPtr(o.pulsarTLSEnableHostnameVerification)
	}

	if o.flagChanged(cmd, flagPulsarTLSTrustSecretName) || o.flagChanged(cmd, flagPulsarTLSTrustSecretKey) {
		ref := cloneSecretKeyRef(cfg.TrustCertsSecretRef)
		if ref == nil {
			ref = &registry.SecretKeyRef{}
		}
		if o.flagChanged(cmd, flagPulsarTLSTrustSecretName) {
			ref.Name = strings.TrimSpace(o.pulsarTLSTrustSecretName)
		}
		if o.flagChanged(cmd, flagPulsarTLSTrustSecretKey) {
			ref.Key = strings.TrimSpace(o.pulsarTLSTrustSecretKey)
		}
		if err := validateSecretKeyRef(ref, "Pulsar TLS trust certs"); err != nil {
			return nil, err
		}
		cfg.TrustCertsSecretRef = ref
	}
	if o.flagChanged(cmd, flagPulsarTLSCertSecretName) || o.flagChanged(cmd, flagPulsarTLSCertSecretKey) {
		ref := cloneSecretKeyRef(cfg.ClientCertSecretRef)
		if ref == nil {
			ref = &registry.SecretKeyRef{}
		}
		if o.flagChanged(cmd, flagPulsarTLSCertSecretName) {
			ref.Name = strings.TrimSpace(o.pulsarTLSCertSecretName)
		}
		if o.flagChanged(cmd, flagPulsarTLSCertSecretKey) {
			ref.Key = strings.TrimSpace(o.pulsarTLSCertSecretKey)
		}
		if err := validateSecretKeyRef(ref, "Pulsar TLS client certificate"); err != nil {
			return nil, err
		}
		cfg.ClientCertSecretRef = ref
	}
	if o.flagChanged(cmd, flagPulsarTLSKeySecretName) || o.flagChanged(cmd, flagPulsarTLSKeySecretKey) {
		ref := cloneSecretKeyRef(cfg.ClientKeySecretRef)
		if ref == nil {
			ref = &registry.SecretKeyRef{}
		}
		if o.flagChanged(cmd, flagPulsarTLSKeySecretName) {
			ref.Name = strings.TrimSpace(o.pulsarTLSKeySecretName)
		}
		if o.flagChanged(cmd, flagPulsarTLSKeySecretKey) {
			ref.Key = strings.TrimSpace(o.pulsarTLSKeySecretKey)
		}
		if err := validateSecretKeyRef(ref, "Pulsar TLS client key"); err != nil {
			return nil, err
		}
		cfg.ClientKeySecretRef = ref
	}

	return cfg, nil
}

func (o *connectionMutationOptions) buildKafkaAuth(cmd *cobra.Command, existing map[string]interface{}) (map[string]interface{}, error) {
	existingCfg := kafkaAuthenticationConfig{}
	var err error
	if len(existing) > 0 {
		existingCfg, err = decodeMapObject[kafkaAuthenticationConfig](existing)
		if err != nil {
			return nil, fmt.Errorf("failed to decode existing Kafka auth config: %w", err)
		}
	}

	existingMode := detectKafkaAuthMode(existingCfg)
	targetMode, err := o.resolveKafkaAuthMode(cmd, existingMode)
	if err != nil {
		return nil, err
	}
	if targetMode == authTypeNone {
		return nil, nil
	}

	nextCfg := kafkaAuthenticationConfig{}
	switch targetMode {
	case kafkaAuthTypePlain:
		cfg := kafkaPlainAuthConfig{}
		if existingMode == kafkaAuthTypePlain && existingCfg.PlainAuthConfig != nil {
			cfg = *existingCfg.PlainAuthConfig
		}
		if o.flagChanged(cmd, flagKafkaPlainSecretName) {
			cfg.SecretName = strings.TrimSpace(o.kafkaPlainSecretName)
		}
		if o.flagChanged(cmd, flagKafkaPlainUsernameKey) {
			cfg.UsernameKey = strings.TrimSpace(o.kafkaPlainUsernameKey)
		}
		if o.flagChanged(cmd, flagKafkaPlainPasswordKey) {
			cfg.PasswordKey = strings.TrimSpace(o.kafkaPlainPasswordKey)
		}
		if strings.TrimSpace(cfg.SecretName) == "" || strings.TrimSpace(cfg.UsernameKey) == "" || strings.TrimSpace(cfg.PasswordKey) == "" {
			return nil, fmt.Errorf("Kafka plain auth requires --%s, --%s, and --%s", flagKafkaPlainSecretName, flagKafkaPlainUsernameKey, flagKafkaPlainPasswordKey)
		}
		nextCfg.PlainAuthConfig = &cfg
	case kafkaAuthTypeScram:
		cfg := kafkaScramAuthConfig{}
		if existingMode == kafkaAuthTypeScram && existingCfg.ScramAuthConfig != nil {
			cfg = *existingCfg.ScramAuthConfig
		}
		if o.flagChanged(cmd, flagKafkaScramSecretName) {
			cfg.SecretName = strings.TrimSpace(o.kafkaScramSecretName)
		}
		if o.flagChanged(cmd, flagKafkaScramUsernameKey) {
			cfg.UsernameKey = strings.TrimSpace(o.kafkaScramUsernameKey)
		}
		if o.flagChanged(cmd, flagKafkaScramPasswordKey) {
			cfg.PasswordKey = strings.TrimSpace(o.kafkaScramPasswordKey)
		}
		if o.flagChanged(cmd, flagKafkaScramHash) {
			hash, err := normalizeKafkaScramHash(o.kafkaScramHash)
			if err != nil {
				return nil, err
			}
			cfg.HashAlgorithm = hash
		} else if strings.TrimSpace(cfg.HashAlgorithm) == "" {
			cfg.HashAlgorithm = "SHA-512"
		}
		if strings.TrimSpace(cfg.SecretName) == "" || strings.TrimSpace(cfg.UsernameKey) == "" || strings.TrimSpace(cfg.PasswordKey) == "" {
			return nil, fmt.Errorf("Kafka SCRAM auth requires --%s, --%s, and --%s", flagKafkaScramSecretName, flagKafkaScramUsernameKey, flagKafkaScramPasswordKey)
		}
		nextCfg.ScramAuthConfig = &cfg
	case authTypeOAuth2:
		cfg := pulsarOAuth2Config{}
		if existingMode == authTypeOAuth2 && existingCfg.OAuth2Config != nil {
			cfg = *existingCfg.OAuth2Config
		}
		if o.flagChanged(cmd, flagKafkaOAuth2IssuerURL) {
			cfg.IssuerURL = strings.TrimSpace(o.kafkaOAuth2IssuerURL)
		}
		if o.flagChanged(cmd, flagKafkaOAuth2Audience) {
			cfg.Audience = strings.TrimSpace(o.kafkaOAuth2Audience)
		}
		if o.flagChanged(cmd, flagKafkaOAuth2Scope) {
			cfg.Scope = strings.TrimSpace(o.kafkaOAuth2Scope)
		}
		if o.flagChanged(cmd, flagKafkaOAuth2SecretName) {
			cfg.KeySecretName = strings.TrimSpace(o.kafkaOAuth2SecretName)
		}
		if o.flagChanged(cmd, flagKafkaOAuth2SecretKey) {
			cfg.KeySecretKey = strings.TrimSpace(o.kafkaOAuth2SecretKey)
		}
		if strings.TrimSpace(cfg.KeySecretName) == "" || strings.TrimSpace(cfg.KeySecretKey) == "" {
			return nil, fmt.Errorf("Kafka OAuth2 requires both --%s and --%s", flagKafkaOAuth2SecretName, flagKafkaOAuth2SecretKey)
		}
		nextCfg.OAuth2Config = &cfg
	case authTypeGeneric:
		cfg := genericAuthConfig{}
		if existingMode == authTypeGeneric && existingCfg.GenericAuth != nil {
			cfg = *existingCfg.GenericAuth
		}
		if o.flagChanged(cmd, flagKafkaGenericAuthPlugin) {
			cfg.ClientAuthenticationPlugin = strings.TrimSpace(o.kafkaGenericAuthPlugin)
		}
		if o.flagChanged(cmd, flagKafkaGenericAuthParams) {
			cfg.ClientAuthenticationParameters = strings.TrimSpace(o.kafkaGenericAuthParams)
		}
		if strings.TrimSpace(cfg.ClientAuthenticationPlugin) == "" || strings.TrimSpace(cfg.ClientAuthenticationParameters) == "" {
			return nil, fmt.Errorf("Kafka generic auth requires both --%s and --%s", flagKafkaGenericAuthPlugin, flagKafkaGenericAuthParams)
		}
		nextCfg.GenericAuth = &cfg
	default:
		return existing, nil
	}

	return encodeMapObject(nextCfg)
}

func (o *connectionMutationOptions) buildKafkaTLS(cmd *cobra.Command, existing map[string]interface{}) (map[string]interface{}, error) {
	if !o.changedAny(cmd, kafkaTLSFlagNames...) {
		return cloneInterfaceMap(existing), nil
	}
	if o.flagChanged(cmd, flagKafkaTLSEnabled) && !o.kafkaTLSEnabled {
		if o.changedAny(cmd, kafkaTLSSubFlagNames...) {
			return nil, fmt.Errorf("Kafka TLS sub-flags cannot be used with --%s=false", flagKafkaTLSEnabled)
		}
		return nil, nil
	}

	cfg := kafkaTLSConfig{}
	var err error
	if len(existing) > 0 {
		cfg, err = decodeMapObject[kafkaTLSConfig](existing)
		if err != nil {
			return nil, fmt.Errorf("failed to decode existing Kafka TLS config: %w", err)
		}
	}
	if o.flagChanged(cmd, flagKafkaTLSEnabled) {
		cfg.Enabled = o.kafkaTLSEnabled
	}
	if !o.flagChanged(cmd, flagKafkaTLSEnabled) && o.changedAny(cmd, kafkaTLSSubFlagNames...) {
		cfg.Enabled = true
	}

	if o.changedAny(cmd, flagKafkaTLSTrustSecretName, flagKafkaTLSTrustFileKey, flagKafkaTLSTrustPasswordKey, flagKafkaTLSTrustType) {
		trust := cfg.TrustStoreConfig
		if trust == nil {
			trust = &kafkaTrustStoreConfig{}
		}
		if o.flagChanged(cmd, flagKafkaTLSTrustSecretName) {
			trust.SecretName = strings.TrimSpace(o.kafkaTLSTrustSecretName)
		}
		if o.flagChanged(cmd, flagKafkaTLSTrustFileKey) {
			trust.FileKey = strings.TrimSpace(o.kafkaTLSTrustFileKey)
		}
		if o.flagChanged(cmd, flagKafkaTLSTrustPasswordKey) {
			trust.PasswordKey = strings.TrimSpace(o.kafkaTLSTrustPasswordKey)
		}
		if o.flagChanged(cmd, flagKafkaTLSTrustType) {
			trust.Type = strings.ToUpper(strings.TrimSpace(o.kafkaTLSTrustType))
		}
		if strings.TrimSpace(trust.SecretName) == "" {
			return nil, fmt.Errorf("Kafka TLS trust store requires --%s", flagKafkaTLSTrustSecretName)
		}
		cfg.TrustStoreConfig = trust
	}

	if o.changedAny(cmd, flagKafkaTLSKeySecretName, flagKafkaTLSKeyFileKey, flagKafkaTLSKeyPasswordKey, flagKafkaTLSKeyKeyPasswordKey, flagKafkaTLSKeyType) {
		key := cfg.KeyStoreConfig
		if key == nil {
			key = &kafkaKeyStoreConfig{}
		}
		if o.flagChanged(cmd, flagKafkaTLSKeySecretName) {
			key.SecretName = strings.TrimSpace(o.kafkaTLSKeySecretName)
		}
		if o.flagChanged(cmd, flagKafkaTLSKeyFileKey) {
			key.FileKey = strings.TrimSpace(o.kafkaTLSKeyFileKey)
		}
		if o.flagChanged(cmd, flagKafkaTLSKeyPasswordKey) {
			key.PasswordKey = strings.TrimSpace(o.kafkaTLSKeyPasswordKey)
		}
		if o.flagChanged(cmd, flagKafkaTLSKeyKeyPasswordKey) {
			key.KeyPasswordKey = strings.TrimSpace(o.kafkaTLSKeyKeyPasswordKey)
		}
		if o.flagChanged(cmd, flagKafkaTLSKeyType) {
			key.Type = strings.ToUpper(strings.TrimSpace(o.kafkaTLSKeyType))
		}
		if strings.TrimSpace(key.SecretName) == "" {
			return nil, fmt.Errorf("Kafka TLS key store requires --%s", flagKafkaTLSKeySecretName)
		}
		cfg.KeyStoreConfig = key
	}

	return encodeMapObject(cfg)
}

func (o *connectionMutationOptions) resolvePulsarAuthMode(cmd *cobra.Command, existingMode string) (string, error) {
	explicitMode := strings.TrimSpace(o.pulsarAuthType)
	if o.flagChanged(cmd, flagPulsarAuthType) {
		if !isAllowedValue(explicitMode, authTypeNone, authTypeToken, authTypeOAuth2, authTypeGeneric) {
			return "", fmt.Errorf("--%s must be one of: none, token, oauth2, generic", flagPulsarAuthType)
		}
	}

	changedMode, err := resolveChangedAuthMode(cmd,
		map[string][]string{
			authTypeToken:   pulsarTokenFlagNames,
			authTypeOAuth2:  pulsarOAuth2FlagNames,
			authTypeGeneric: pulsarGenericAuthFlagNames,
		},
	)
	if err != nil {
		return "", fmt.Errorf("invalid Pulsar auth flags: %w", err)
	}

	if explicitMode == authTypeNone {
		if changedMode != "" {
			return "", fmt.Errorf("Pulsar auth flags cannot be combined with --%s=none", flagPulsarAuthType)
		}
		return authTypeNone, nil
	}
	if explicitMode != "" {
		if changedMode != "" && changedMode != explicitMode {
			return "", fmt.Errorf("Pulsar auth flags for %s cannot be combined with --%s=%s", changedMode, flagPulsarAuthType, explicitMode)
		}
		return explicitMode, nil
	}
	if changedMode == "" {
		if existingMode == "" {
			return authTypeNone, nil
		}
		return existingMode, nil
	}
	if existingMode != "" && existingMode != authTypeNone && existingMode != changedMode {
		return "", fmt.Errorf("changing Pulsar auth mode from %s to %s requires --%s=%s", existingMode, changedMode, flagPulsarAuthType, changedMode)
	}
	return changedMode, nil
}

func (o *connectionMutationOptions) resolveKafkaAuthMode(cmd *cobra.Command, existingMode string) (string, error) {
	explicitMode := strings.TrimSpace(o.kafkaAuthType)
	if o.flagChanged(cmd, flagKafkaAuthType) {
		if !isAllowedValue(explicitMode, authTypeNone, kafkaAuthTypePlain, kafkaAuthTypeScram, authTypeOAuth2, authTypeGeneric) {
			return "", fmt.Errorf("--%s must be one of: none, plain, scram, oauth2, generic", flagKafkaAuthType)
		}
	}

	changedMode, err := resolveChangedAuthMode(cmd,
		map[string][]string{
			kafkaAuthTypePlain: kafkaPlainFlagNames,
			kafkaAuthTypeScram: kafkaScramFlagNames,
			authTypeOAuth2:     kafkaOAuth2FlagNames,
			authTypeGeneric:    kafkaGenericAuthFlagNames,
		},
	)
	if err != nil {
		return "", fmt.Errorf("invalid Kafka auth flags: %w", err)
	}

	if explicitMode == authTypeNone {
		if changedMode != "" {
			return "", fmt.Errorf("Kafka auth flags cannot be combined with --%s=none", flagKafkaAuthType)
		}
		return authTypeNone, nil
	}
	if explicitMode != "" {
		if changedMode != "" && changedMode != explicitMode {
			return "", fmt.Errorf("Kafka auth flags for %s cannot be combined with --%s=%s", changedMode, flagKafkaAuthType, explicitMode)
		}
		return explicitMode, nil
	}
	if changedMode == "" {
		if existingMode == "" {
			return authTypeNone, nil
		}
		return existingMode, nil
	}
	if existingMode != "" && existingMode != authTypeNone && existingMode != changedMode {
		return "", fmt.Errorf("changing Kafka auth mode from %s to %s requires --%s=%s", existingMode, changedMode, flagKafkaAuthType, changedMode)
	}
	return changedMode, nil
}

func validateBuiltConnectionSpec(spec registry.ConnectionSpec) error {
	switch spec.Type {
	case registry.ConnectionTypePulsar:
		if spec.Pulsar == nil {
			return fmt.Errorf("Pulsar connection spec is required")
		}
		if strings.TrimSpace(spec.Pulsar.ServiceURL) == "" && strings.TrimSpace(spec.Pulsar.AdminURL) == "" {
			return fmt.Errorf("Pulsar connection requires at least one of --%s or --%s", flagPulsarServiceURL, flagPulsarAdminURL)
		}
	case registry.ConnectionTypeKafka:
		if spec.Kafka == nil {
			return fmt.Errorf("Kafka connection spec is required")
		}
		if strings.TrimSpace(spec.Kafka.BootstrapServers) == "" {
			return fmt.Errorf("Kafka connection requires --%s", flagKafkaBootstrapServers)
		}
	case registry.ConnectionTypeOther:
		if spec.Other == nil {
			return fmt.Errorf("Other connection spec is required")
		}
		if strings.TrimSpace(spec.Other.Endpoint) == "" {
			return fmt.Errorf("Other connection requires --%s", flagOtherEndpoint)
		}
	default:
		return fmt.Errorf("unsupported connection type %q", spec.Type)
	}
	return nil
}

func parseConnectionType(value string) (registry.ConnectionType, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case string(registry.ConnectionTypePulsar):
		return registry.ConnectionTypePulsar, nil
	case string(registry.ConnectionTypeKafka):
		return registry.ConnectionTypeKafka, nil
	case string(registry.ConnectionTypeOther):
		return registry.ConnectionTypeOther, nil
	default:
		return "", fmt.Errorf("--%s must be one of: pulsar, kafka, other", flagConnectionType)
	}
}

func parseOtherProperties(entries []string) (map[string]string, error) {
	properties := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			return nil, fmt.Errorf("--%s entries must use key=value format", flagOtherProperty)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("--%s entries must have a non-empty key", flagOtherProperty)
		}
		properties[key] = strings.TrimSpace(value)
	}
	return properties, nil
}

func validateSecretKeyRef(ref *registry.SecretKeyRef, label string) error {
	if ref == nil {
		return nil
	}
	if strings.TrimSpace(ref.Name) == "" || strings.TrimSpace(ref.Key) == "" {
		return fmt.Errorf("%s requires both secret name and key", label)
	}
	return nil
}

func detectPulsarAuthMode(auth *registry.PulsarAuthConfig) string {
	if auth == nil {
		return authTypeNone
	}
	if auth.Token != nil {
		return authTypeToken
	}
	if len(auth.OAuth2) > 0 {
		return authTypeOAuth2
	}
	if len(auth.GenericAuth) > 0 {
		return authTypeGeneric
	}
	return authTypeNone
}

func detectKafkaAuthMode(auth kafkaAuthenticationConfig) string {
	switch {
	case auth.PlainAuthConfig != nil:
		return kafkaAuthTypePlain
	case auth.ScramAuthConfig != nil:
		return kafkaAuthTypeScram
	case auth.OAuth2Config != nil:
		return authTypeOAuth2
	case auth.GenericAuth != nil:
		return authTypeGeneric
	default:
		return authTypeNone
	}
}

func resolveChangedAuthMode(cmd *cobra.Command, groups map[string][]string) (string, error) {
	active := ""
	for mode, flags := range groups {
		if changedAnyFlags(cmd, flags...) {
			if active != "" {
				return "", fmt.Errorf("multiple auth modes were specified")
			}
			active = mode
		}
	}
	return active, nil
}

func normalizeKafkaScramHash(value string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "sha-512":
		return "SHA-512", nil
	case "sha-256":
		return "SHA-256", nil
	default:
		return "", fmt.Errorf("--%s must be one of: sha-256, sha-512", flagKafkaScramHash)
	}
}

func decodeMapObject[T any](input map[string]interface{}) (T, error) {
	var target T
	if len(input) == 0 {
		return target, nil
	}

	data, err := json.Marshal(input)
	if err != nil {
		return target, err
	}
	if err := json.Unmarshal(data, &target); err != nil {
		return target, err
	}
	return target, nil
}

func encodeMapObject(value any) (map[string]interface{}, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func clonePulsarConnectionConfig(existing *registry.PulsarConnectionConfig) *registry.PulsarConnectionConfig {
	if existing == nil {
		return nil
	}
	return &registry.PulsarConnectionConfig{
		ServiceURL:     existing.ServiceURL,
		AdminURL:       existing.AdminURL,
		Authentication: clonePulsarAuth(existing.Authentication),
		TLS:            clonePulsarTLSConfig(existing.TLS),
	}
}

func clonePulsarAuth(existing *registry.PulsarAuthConfig) *registry.PulsarAuthConfig {
	if existing == nil {
		return nil
	}
	return &registry.PulsarAuthConfig{
		Token:       cloneSecretKeyRef(existing.Token),
		OAuth2:      cloneInterfaceMap(existing.OAuth2),
		GenericAuth: cloneInterfaceMap(existing.GenericAuth),
	}
}

func clonePulsarTLSConfig(existing *registry.PulsarTLSConfig) *registry.PulsarTLSConfig {
	if existing == nil {
		return nil
	}
	var hostnameVerification *bool
	if existing.EnableHostnameVerification != nil {
		hostnameVerification = boolPtr(*existing.EnableHostnameVerification)
	}
	return &registry.PulsarTLSConfig{
		Enabled:                    existing.Enabled,
		AllowInsecureConnection:    existing.AllowInsecureConnection,
		EnableHostnameVerification: hostnameVerification,
		TrustCertsSecretRef:        cloneSecretKeyRef(existing.TrustCertsSecretRef),
		ClientCertSecretRef:        cloneSecretKeyRef(existing.ClientCertSecretRef),
		ClientKeySecretRef:         cloneSecretKeyRef(existing.ClientKeySecretRef),
	}
}

func cloneKafkaConnectionConfig(existing *registry.KafkaConnectionConfig) *registry.KafkaConnectionConfig {
	if existing == nil {
		return nil
	}
	return &registry.KafkaConnectionConfig{
		BootstrapServers: existing.BootstrapServers,
		TLS:              cloneInterfaceMap(existing.TLS),
		Authentication:   cloneInterfaceMap(existing.Authentication),
	}
}

func cloneOtherConnectionConfig(existing *registry.OtherConnectionConfig) *registry.OtherConnectionConfig {
	if existing == nil {
		return nil
	}
	properties := make(map[string]string, len(existing.Properties))
	for key, value := range existing.Properties {
		properties[key] = value
	}
	return &registry.OtherConnectionConfig{
		Endpoint:   existing.Endpoint,
		Properties: properties,
		SecretRef:  cloneSecretKeyRef(existing.SecretRef),
	}
}

func mergeStringMap(base map[string]string, overrides map[string]string) map[string]string {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}

	merged := make(map[string]string, len(base)+len(overrides))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}

	return merged
}

func cloneSecretKeyRef(existing *registry.SecretKeyRef) *registry.SecretKeyRef {
	if existing == nil {
		return nil
	}
	return &registry.SecretKeyRef{
		Name: existing.Name,
		Key:  existing.Key,
	}
}

func cloneInterfaceMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return map[string]interface{}{}
	}
	var cloned map[string]interface{}
	if err := json.Unmarshal(data, &cloned); err != nil {
		return map[string]interface{}{}
	}
	return cloned
}

func boolPtr(value bool) *bool {
	return &value
}

func isAllowedValue(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func (o *connectionMutationOptions) flagChanged(cmd *cobra.Command, name string) bool {
	return cmd.Flags().Changed(name)
}

func (o *connectionMutationOptions) changedAny(cmd *cobra.Command, names ...string) bool {
	return changedAnyFlags(cmd, names...)
}

func changedAnyFlags(cmd *cobra.Command, names ...string) bool {
	for _, name := range names {
		if cmd.Flags().Changed(name) {
			return true
		}
	}
	return false
}
