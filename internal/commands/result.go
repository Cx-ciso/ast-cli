package commands

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/checkmarx/ast-cli/internal/commands/util"
	"github.com/checkmarx/ast-cli/internal/commands/util/printer"
	"github.com/checkmarx/ast-cli/internal/logger"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	commonParams "github.com/checkmarx/ast-cli/internal/params"

	"github.com/checkmarx/ast-cli/internal/wrappers"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	failedCreatingSummary     = "Failed creating summary"
	failedGettingScan         = "Failed getting scan"
	failedListingResults      = "Failed listing results"
	failedListingCodeBashing  = "Failed codebashing link"
	mediumLabel               = "medium"
	highLabel                 = "high"
	lowLabel                  = "low"
	infoLabel                 = "info"
	sonarTypeLabel            = "_sonar"
	glSastTypeLobel           = ".gl-sast-report"
	directoryPermission       = 0700
	infoSonar                 = "INFO"
	lowSonar                  = "MINOR"
	mediumSonar               = "MAJOR"
	highSonar                 = "CRITICAL"
	infoLowSarif              = "note"
	mediumSarif               = "warning"
	highSarif                 = "error"
	vulnerabilitySonar        = "VULNERABILITY"
	infoCx                    = "INFO"
	lowCx                     = "LOW"
	mediumCx                  = "MEDIUM"
	highCx                    = "HIGH"
	codeBashingKey            = "cb-url"
	failedGettingBfl          = "Failed getting BFL"
	notAvailableString        = "N/A"
	notAvailableNumber        = -1
	defaultPaddingSize        = -13
	defaultResultsPaddingSize = -15
	scanPendingMessage        = "Scan triggered in asynchronous mode or still running. Click more details to get the full status."
	directDependencyType      = "Direct Dependency"
	indirectDependencyType    = "Transitive Dependency"
	startedStatus             = "started"
	requestedStatus           = "requested"
	completedStatus           = "completed"
	exportingStatus           = "Exporting"
	pendingStatus             = "Pending"
	pdfToEmailFlagDescription = "Send the PDF report to the specified email address." +
		" Use \",\" as the delimiter for multiple emails"
	pdfOptionsFlagDescription = "Sections to generate PDF report. Available options: Iac-Security,Sast,Sca," +
		defaultPdfOptionsDataSections
	sbomReportFlagDescription               = "Sections to generate SBOM report. Available options: CycloneDxJson,CycloneDxXml,SpdxJson"
	delayValueForReport                     = 10
	reportNameScanReport                    = "scan-report"
	reportTypeEmail                         = "email"
	defaultPdfOptionsDataSections           = "ScanSummary,ExecutiveSummary,ScanResults"
	defaultSbomOption                       = "CycloneDxJson"
	exploitablePathFlagDescription          = "Enable or disable exploitable path in scan. Available options: true,false"
	scaLastScanTimeFlagDescription          = "SCA last scan time. Available options: integer above 1"
	projectPrivatePackageFlagDescription    = "Enable or disable project private package. Available options: true,false"
	scaPrivatePackageVersionFlagDescription = "SCA project private package version. Example: 0.1.1"
	policeManagementNoneStatus              = "none"
	apiDocumentationFlagDescription         = "Swagger folder/file filter for API-Security scan. Example: ./swagger.json"
	summaryCreatedAtLayout                  = "2006-01-02, 15:04:05"
	glTimeFormat                            = "2006-01-02T15:04:05"
	sarifNodeFileLength                     = 2
	fixLabel                                = "fix"
	redundantLabel                          = "redundant"
)

var summaryFormats = []string{
	printer.FormatSummaryConsole,
	printer.FormatSummary,
	printer.FormatSummaryJSON,
	printer.FormatPDF,
	printer.FormatSummaryMarkdown,
	printer.FormatSbom,
	printer.FormatGL,
}

var filterResultsListFlagUsage = fmt.Sprintf(
	"Filter the list of results. Use ';' as the delimiter for arrays. Available filters are: %s",
	strings.Join(
		[]string{
			commonParams.ScanIDQueryParam,
			commonParams.LimitQueryParam,
			commonParams.OffsetQueryParam,
			commonParams.SortQueryParam,
			commonParams.IncludeNodesQueryParam,
			commonParams.NodeIDsQueryParam,
			commonParams.QueryQueryParam,
			commonParams.GroupQueryParam,
			commonParams.StatusQueryParam,
			commonParams.SeverityQueryParam,
			commonParams.StateQueryParam,
		}, ",",
	),
)

var securities = map[string]string{
	infoCx:   "3.5",
	lowCx:    "6.5",
	mediumCx: "8.5",
	highCx:   "9.5",
}

// Match cx severity with sonar severity
var sonarSeverities = map[string]string{
	infoCx:   infoSonar,
	lowCx:    lowSonar,
	mediumCx: mediumSonar,
	highCx:   highSonar,
}

func NewResultsCommand(
	resultsWrapper wrappers.ResultsWrapper,
	scanWrapper wrappers.ScansWrapper,
	resultsSbomWrapper wrappers.ResultsSbomWrapper,
	resultsPdfReportsWrapper wrappers.ResultsPdfWrapper,
	codeBashingWrapper wrappers.CodeBashingWrapper,
	bflWrapper wrappers.BflWrapper,
	risksOverviewWrapper wrappers.RisksOverviewWrapper,
	policyWrapper wrappers.PolicyWrapper,
) *cobra.Command {
	resultCmd := &cobra.Command{
		Use:   "results",
		Short: "Retrieve results",
		Annotations: map[string]string{
			"command:doc": heredoc.Doc(
				`
				https://checkmarx.com/resource/documents/en/34965-68640-results.html
			`,
			),
		},
	}
	showResultCmd := resultShowSubCommand(resultsWrapper, scanWrapper, resultsSbomWrapper, resultsPdfReportsWrapper, risksOverviewWrapper, policyWrapper)
	codeBashingCmd := resultCodeBashing(codeBashingWrapper)
	bflResultCmd := resultBflSubCommand(bflWrapper)
	resultCmd.AddCommand(
		showResultCmd, bflResultCmd, codeBashingCmd,
	)
	return resultCmd
}

func resultShowSubCommand(
	resultsWrapper wrappers.ResultsWrapper,
	scanWrapper wrappers.ScansWrapper,
	resultsSbomWrapper wrappers.ResultsSbomWrapper,
	resultsPdfReportsWrapper wrappers.ResultsPdfWrapper,
	risksOverviewWrapper wrappers.RisksOverviewWrapper,
	policyWrapper wrappers.PolicyWrapper,
) *cobra.Command {
	resultShowCmd := &cobra.Command{
		Use:   "show",
		Short: "Show results of a scan",
		Long:  "The show command enables the ability to show results about a requested scan in Checkmarx One.",
		Example: heredoc.Doc(
			`
			$ cx results show --scan-id <scan Id>
		`,
		),
		RunE: runGetResultCommand(resultsWrapper, scanWrapper, resultsSbomWrapper, resultsPdfReportsWrapper, risksOverviewWrapper, policyWrapper),
	}
	addScanIDFlag(resultShowCmd, "ID to report on.")
	addResultFormatFlag(
		resultShowCmd,
		printer.FormatJSON,
		printer.FormatSummary,
		printer.FormatSummaryConsole,
		printer.FormatSarif,
		printer.FormatSummaryJSON,
		printer.FormatSbom,
		printer.FormatPDF,
		printer.FormatSummaryMarkdown,
		printer.FormatGL,
	)
	resultShowCmd.PersistentFlags().String(commonParams.ReportFormatPdfToEmailFlag, "", pdfToEmailFlagDescription)
	resultShowCmd.PersistentFlags().String(commonParams.ReportSbomFormatFlag, defaultSbomOption, sbomReportFlagDescription)
	resultShowCmd.PersistentFlags().String(commonParams.ReportFormatPdfOptionsFlag, defaultPdfOptionsDataSections, pdfOptionsFlagDescription)
	resultShowCmd.PersistentFlags().String(commonParams.TargetFlag, "cx_result", "Output file")
	resultShowCmd.PersistentFlags().String(commonParams.TargetPathFlag, ".", "Output Path")
	resultShowCmd.PersistentFlags().StringSlice(commonParams.FilterFlag, []string{}, filterResultsListFlagUsage)

	resultShowCmd.PersistentFlags().Int(commonParams.RetrySBOMFlag, commonParams.RetrySBOMDefault, commonParams.RetrySBOMUsage)

	// Temporary flag until SCA supports new api
	resultShowCmd.PersistentFlags().Bool(commonParams.ReportSbomFormatLocalFlowFlag, false, "")
	_ = resultShowCmd.PersistentFlags().MarkHidden(commonParams.ReportSbomFormatLocalFlowFlag)
	resultShowCmd.PersistentFlags().IntP(
		commonParams.WaitDelayFlag,
		"",
		commonParams.WaitDelayDefault,
		"Polling wait time in seconds",
	)
	resultShowCmd.PersistentFlags().Int(
		commonParams.PolicyTimeoutFlag,
		commonParams.ResultPolicyDefaultTimeout,
		"Cancel the policy evaluation and fail after the timeout in minutes",
	)
	resultShowCmd.PersistentFlags().Bool(commonParams.IgnorePolicyFlag, false, "Do not evaluate policies")
	resultShowCmd.PersistentFlags().Bool(commonParams.SastRedundancyFlag, false,
		"Populate SAST results 'data.redundancy' with values '"+fixLabel+"' (to fix) or '"+redundantLabel+"' (no need to fix)")
	return resultShowCmd
}

func resultBflSubCommand(bflWrapper wrappers.BflWrapper) *cobra.Command {
	resultBflCmd := &cobra.Command{
		Use:   "bfl",
		Short: "Show best fix location for a query id within the scan result.",
		Long:  "The bfl command enables the ability to show best fix location for a querid within the scan result.",
		Example: heredoc.Doc(
			`
			$ cx results bfl --scan-id <scan Id> --query-id <query Id>
		`,
		),
		RunE: runGetBestFixLocationCommand(bflWrapper),
	}
	addScanIDFlag(resultBflCmd, "ID to report on.")
	addQueryIDFlag(resultBflCmd, "Query Id from the result.")
	addFormatFlag(resultBflCmd, printer.FormatList, printer.FormatJSON)

	markFlagAsRequired(resultBflCmd, commonParams.ScanIDFlag)
	markFlagAsRequired(resultBflCmd, commonParams.QueryIDFlag)

	return resultBflCmd
}

func runGetBestFixLocationCommand(bflWrapper wrappers.BflWrapper) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		var bflResponseModel *wrappers.BFLResponseModel
		var errorModel *wrappers.WebError
		var err error

		scanID, _ := cmd.Flags().GetString(commonParams.ScanIDFlag)
		queryID, _ := cmd.Flags().GetString(commonParams.QueryIDFlag)

		scanIds := strings.Split(scanID, ",")
		if len(scanIds) > 1 {
			return errors.Errorf("%s", "Multiple scan-ids are not allowed.")
		}
		queryIds := strings.Split(queryID, ",")
		if len(queryIds) > 1 {
			return errors.Errorf("%s", "Multiple query-ids are not allowed.")
		}

		params := make(map[string]string)
		params[commonParams.ScanIDQueryParam] = scanID
		params[commonParams.QueryIDQueryParam] = queryID

		bflResponseModel, errorModel, err = bflWrapper.GetBflByScanIDAndQueryID(params)

		if err != nil {
			return errors.Wrapf(err, "%s", failedGettingBfl)
		}

		// Checking the response
		if errorModel != nil {
			return errors.Errorf("%s: CODE: %d, %s", failedGettingBfl, errorModel.Code, errorModel.Message)
		} else if bflResponseModel != nil {
			err = printByFormat(cmd, toBflView(*bflResponseModel))
			if err != nil {
				return err
			}
		}

		return nil
	}
}

func toBflView(bflResponseModel wrappers.BFLResponseModel) []wrappers.ScanResultNode {
	if (bflResponseModel.TotalCount) > 0 {
		views := make([]wrappers.ScanResultNode, bflResponseModel.TotalCount)

		for i := 0; i < bflResponseModel.TotalCount; i++ {
			views[i] = wrappers.ScanResultNode{
				Name:       bflResponseModel.Trees[i].BFL.Name,
				FileName:   bflResponseModel.Trees[i].BFL.FileName,
				FullName:   bflResponseModel.Trees[i].BFL.FullName,
				Column:     bflResponseModel.Trees[i].BFL.Column,
				Length:     bflResponseModel.Trees[i].BFL.Length,
				Line:       bflResponseModel.Trees[i].BFL.Line,
				MethodLine: bflResponseModel.Trees[i].BFL.MethodLine,
				Method:     bflResponseModel.Trees[i].BFL.Method,
				DomType:    bflResponseModel.Trees[i].BFL.DomType,
			}
		}
		return views
	}
	views := make([]wrappers.ScanResultNode, 0)
	return views
}

func resultCodeBashing(codeBashingWrapper wrappers.CodeBashingWrapper) *cobra.Command {
	// Create a codeBashing wrapper
	resultCmd := &cobra.Command{
		Use:   "codebashing",
		Short: "Get codebashing lesson link",
		Long:  "The codebashing command enables the ability to retrieve the link about a specific vulnerability.",
		Example: heredoc.Doc(
			`
			$ cx results codebashing --language <string> --vulnerability-type <string> --cwe-id <string> --format <string>
		`,
		),
		RunE: runGetCodeBashingCommand(codeBashingWrapper),
	}
	resultCmd.PersistentFlags().String(commonParams.LanguageFlag, "", "Language of the vulnerability")
	err := resultCmd.MarkPersistentFlagRequired(commonParams.LanguageFlag)
	if err != nil {
		log.Fatal(err)
	}
	resultCmd.PersistentFlags().String(commonParams.VulnerabilityTypeFlag, "", "Vulnerability type")
	err = resultCmd.MarkPersistentFlagRequired(commonParams.VulnerabilityTypeFlag)
	if err != nil {
		log.Fatal(err)
	}
	resultCmd.PersistentFlags().String(commonParams.CweIDFlag, "", "CWE ID for the vulnerability")
	err = resultCmd.MarkPersistentFlagRequired(commonParams.CweIDFlag)
	if err != nil {
		log.Fatal(err)
	}
	addFormatFlag(resultCmd, printer.FormatJSON, printer.FormatTable, printer.FormatList)
	return resultCmd
}

func convertScanToResultsSummary(scanInfo *wrappers.ScanResponseModel, resultsWrapper wrappers.ResultsWrapper) (*wrappers.ResultSummary, error) {
	if scanInfo == nil {
		return nil, errors.New(failedCreatingSummary)
	}

	sastIssues := 0
	scaIssues := 0
	kicsIssues := 0
	if len(scanInfo.StatusDetails) > 0 {
		for _, statusDetailItem := range scanInfo.StatusDetails {
			if statusDetailItem.Status == wrappers.ScanFailed || statusDetailItem.Status == wrappers.ScanCanceled {
				if statusDetailItem.Name == commonParams.SastType {
					sastIssues = notAvailableNumber
				} else if statusDetailItem.Name == commonParams.ScaType {
					scaIssues = notAvailableNumber
				} else if statusDetailItem.Name == commonParams.KicsType {
					kicsIssues = notAvailableNumber
				}
			}
		}
	}
	summary := &wrappers.ResultSummary{
		ScanID:         scanInfo.ID,
		Status:         string(scanInfo.Status),
		CreatedAt:      scanInfo.CreatedAt.Format("2006-01-02, 15:04:05"),
		ProjectID:      scanInfo.ProjectID,
		RiskStyle:      "",
		RiskMsg:        "",
		HighIssues:     0,
		MediumIssues:   0,
		LowIssues:      0,
		InfoIssues:     0,
		SastIssues:     sastIssues,
		KicsIssues:     kicsIssues,
		ScaIssues:      scaIssues,
		Tags:           scanInfo.Tags,
		ProjectName:    scanInfo.ProjectName,
		BranchName:     scanInfo.Branch,
		EnginesEnabled: scanInfo.Engines,
	}

	baseURI, err := resultsWrapper.GetResultsURL(summary.ProjectID)
	if err != nil {
		return nil, err
	}

	summary.BaseURI = baseURI
	summary.BaseURI = generateScanSummaryURL(summary)
	if isScanPending(summary.Status) {
		summary.ScanInfoMessage = scanPendingMessage
	}

	return summary, nil
}

func summaryReport(
	summary *wrappers.ResultSummary,
	policies *wrappers.PolicyResponseModel,
	risksOverviewWrapper wrappers.RisksOverviewWrapper,
	results *wrappers.ScanResultsCollection,
) (*wrappers.ResultSummary, error) {
	if summary.HasAPISecurity() {
		apiSecRisks, err := getResultsForAPISecScanner(risksOverviewWrapper, summary.ScanID)
		if err != nil {
			return nil, err
		}
		summary.APISecurity = *apiSecRisks
	}

	if policies != nil {
		summary.Policies = filterViolatedRules(*policies)
	}

	enhanceWithScanSummary(summary, results)

	setNotAvailableNumberIfZero(summary, &summary.SastIssues, commonParams.SastType)
	setNotAvailableNumberIfZero(summary, &summary.ScaIssues, commonParams.ScaType)
	setNotAvailableNumberIfZero(summary, &summary.KicsIssues, commonParams.KicsType)
	setRiskMsgAndStyle(summary)

	return summary, nil
}

func setNotAvailableNumberIfZero(summary *wrappers.ResultSummary, counter *int, engineType string) {
	if *counter == 0 && !contains(summary.EnginesEnabled, engineType) {
		*counter = notAvailableNumber
	}
}

func setRiskMsgAndStyle(summary *wrappers.ResultSummary) {
	if summary.HighIssues > 0 {
		summary.RiskStyle = highLabel
		summary.RiskMsg = "High Risk"
	} else if summary.MediumIssues > 0 {
		summary.RiskStyle = mediumLabel
		summary.RiskMsg = "Medium Risk"
	} else if summary.LowIssues > 0 {
		summary.RiskStyle = lowLabel
		summary.RiskMsg = "Low Risk"
	} else if summary.TotalIssues == 0 {
		summary.RiskMsg = "No Risk"
	}
}

func enhanceWithScanSummary(summary *wrappers.ResultSummary, results *wrappers.ScanResultsCollection) {
	for _, result := range results.Results {
		countResult(summary, result)
	}
	summary.TotalIssues = summary.SastIssues + summary.ScaIssues + summary.KicsIssues
}

func writeHTMLSummary(targetFile string, summary *wrappers.ResultSummary) error {
	log.Println("Creating Summary Report: ", targetFile)
	summaryTemp, err := template.New("summaryTemplate").Parse(wrappers.SummaryTemplate(isScanPending(summary.Status)))
	if err == nil {
		f, err := os.Create(targetFile)
		if err == nil {
			_ = summaryTemp.ExecuteTemplate(f, "SummaryTemplate", summary)
			_ = f.Close()
		}
		return err
	}
	return nil
}
func writeMarkdownSummary(targetFile string, data *wrappers.ResultSummary) error {
	log.Println("Creating Markdown Summary Report: ", targetFile)
	tmpl, err := template.New(printer.FormatSummaryMarkdown).Parse(wrappers.SummaryMarkdownTemplate(isScanPending(data.Status)))
	if err != nil {
		return err
	}
	file, err := os.Create(targetFile)
	if err != nil {
		return err
	}
	defer file.Close()

	err = tmpl.Execute(file, &data)
	if err != nil {
		return err
	}
	return nil
}

// nolint: whitespace
func writeConsoleSummary(summary *wrappers.ResultSummary) error {
	if !isScanPending(summary.Status) {
		fmt.Printf("            Scan Summary:                     \n")
		fmt.Printf("              Created At: %s\n", summary.CreatedAt)
		fmt.Printf("              Project Name: %s                        \n", summary.ProjectName)
		fmt.Printf("              Scan ID: %s                             \n\n", summary.ScanID)
		fmt.Printf("            Results Summary:                     \n")
		fmt.Printf(
			"              Risk Level: %s																									 \n",
			summary.RiskMsg,
		)
		fmt.Printf("              --------------------------------------     \n")
		if summary.HasAPISecurity() {
			fmt.Printf(
				"              API Security - Total Detected APIs: %d                       \n",
				summary.APISecurity.APICount)
		}
		if summary.Policies != nil && !strings.EqualFold(summary.Policies.Status, policeManagementNoneStatus) {
			fmt.Printf("              --------------------------------------     \n\n")
			if summary.Policies.BreakBuild {
				fmt.Printf("            Policy Management Violation - Break Build Enabled:                     \n")
			} else {
				fmt.Printf("            Policy Management Violation:                     \n")
			}
			if len(summary.Policies.Polices) > 0 {
				for _, police := range summary.Policies.Polices {
					if len(police.RulesViolated) > 0 {
						fmt.Printf("              Policy: %s | Break Build: %t | Violated Rules: ", police.Name, police.BreakBuild)
						for _, violatedRule := range police.RulesViolated {
							fmt.Printf("%s;", violatedRule)
						}
					}
					fmt.Printf("\n")
				}
			}
			fmt.Printf("\n")
		}

		fmt.Printf("              Total Results: %d                       \n", summary.TotalIssues)
		fmt.Printf("              --------------------------------------     \n")
		fmt.Printf("              |               High: %*d|     \n", defaultResultsPaddingSize, summary.HighIssues)
		fmt.Printf("              |             Medium: %*d|     \n", defaultResultsPaddingSize, summary.MediumIssues)
		fmt.Printf("              |                Low: %*d|     \n", defaultResultsPaddingSize, summary.LowIssues)
		fmt.Printf("              |               Info: %*d|     \n", defaultResultsPaddingSize, summary.InfoIssues)
		fmt.Printf("              --------------------------------------     \n")

		if summary.KicsIssues == notAvailableNumber {
			fmt.Printf("              |         IAC-SECURITY: %*s| \n", defaultPaddingSize, notAvailableString)
		} else {
			fmt.Printf("              |         IAC-SECURITY: %*d| \n", defaultPaddingSize, summary.KicsIssues)
		}
		if summary.SastIssues == notAvailableNumber {
			fmt.Printf("              |                 SAST: %*s| \n", defaultPaddingSize, notAvailableString)
		} else {
			fmt.Printf("              |                 SAST: %*d| \n", defaultPaddingSize, summary.SastIssues)
			if summary.HasAPISecurity() {
				fmt.Printf("              |       APIS WITH RISK: %*d| \n", defaultPaddingSize, summary.APISecurity.TotalRisksCount)
				if summary.HasAPISecurityDocumentation() {
					fmt.Printf("              |   APIS DOCUMENTATION: %*d| \n", defaultPaddingSize, summary.GetAPISecurityDocumentationTotal())
				}
			}
		}
		if summary.ScaIssues == notAvailableNumber {
			fmt.Printf("              |                  SCA: %*s| \n", defaultPaddingSize, notAvailableString)
		} else {
			fmt.Printf("              |                  SCA: %*d| \n", defaultPaddingSize, summary.ScaIssues)
		}
		fmt.Printf("              --------------------------------------     \n\n")
		fmt.Printf("              Checkmarx One - Scan Summary & Details: %s\n", summary.BaseURI)
	} else {
		fmt.Printf("Scan executed in asynchronous mode or still running. Hence, no results generated.\n")
		fmt.Printf("For more information: %s\n", summary.BaseURI)
	}
	return nil
}

func generateScanSummaryURL(summary *wrappers.ResultSummary) string {
	summaryURL := fmt.Sprintf(
		strings.Replace(summary.BaseURI, "overview", "scans?id=%s&branch=%s", 1),
		summary.ScanID, url.QueryEscape(summary.BranchName),
	)
	return summaryURL
}

func runGetResultCommand(
	resultsWrapper wrappers.ResultsWrapper,
	scanWrapper wrappers.ScansWrapper,
	resultsSbomWrapper wrappers.ResultsSbomWrapper,
	resultsPdfReportsWrapper wrappers.ResultsPdfWrapper,
	risksOverviewWrapper wrappers.RisksOverviewWrapper,
	policyWrapper wrappers.PolicyWrapper,
) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		targetFile, _ := cmd.Flags().GetString(commonParams.TargetFlag)
		targetPath, _ := cmd.Flags().GetString(commonParams.TargetPathFlag)
		format, _ := cmd.Flags().GetString(commonParams.TargetFormatFlag)
		formatPdfToEmail, _ := cmd.Flags().GetString(commonParams.ReportFormatPdfToEmailFlag)
		formatPdfOptions, _ := cmd.Flags().GetString(commonParams.ReportFormatPdfOptionsFlag)
		formatSbomOptions, _ := cmd.Flags().GetString(commonParams.ReportSbomFormatFlag)
		useSCALocalFlow, _ := cmd.Flags().GetBool(commonParams.ReportSbomFormatLocalFlowFlag)
		retrySBOM, _ := cmd.Flags().GetInt(commonParams.RetrySBOMFlag)
		sastRedundancy, _ := cmd.Flags().GetBool(commonParams.SastRedundancyFlag)

		scanID, _ := cmd.Flags().GetString(commonParams.ScanIDFlag)
		if scanID == "" {
			return errors.Errorf("%s: Please provide a scan ID", failedListingResults)
		}
		params, err := getFilters(cmd)
		if err != nil {
			return errors.Wrapf(err, "%s", failedListingResults)
		}
		scan, errorModel, scanErr := scanWrapper.GetByID(scanID)
		if scanErr != nil {
			return errors.Wrapf(scanErr, "%s", failedGetting)
		}
		if errorModel != nil {
			return errors.Errorf("%s: CODE: %d, %s", failedGettingScan, errorModel.Code, errorModel.Message)
		}

		policyResponseModel := &wrappers.PolicyResponseModel{}
		policyOverrideFlag, _ := cmd.Flags().GetBool(commonParams.IgnorePolicyFlag)
		waitDelay, _ := cmd.Flags().GetInt(commonParams.WaitDelayFlag)
		if !policyOverrideFlag {
			policyTimeout, _ := cmd.Flags().GetInt(commonParams.PolicyTimeoutFlag)
			if policyTimeout < 0 {
				return errors.Errorf("--%s should be equal or higher than 0", commonParams.PolicyTimeoutFlag)
			}
			policyResponseModel, err = handlePolicyWait(waitDelay, policyTimeout, policyWrapper, scan, cmd)
			if err != nil {
				return err
			}
		} else {
			logger.PrintIfVerbose("Skipping policy evaluation")
		}

		if sastRedundancy {
			params[commonParams.SastRedundancyFlag] = ""
		}

		return CreateScanReport(
			resultsWrapper,
			risksOverviewWrapper,
			resultsSbomWrapper,
			policyResponseModel,
			useSCALocalFlow,
			retrySBOM,
			resultsPdfReportsWrapper,
			scan,
			format,
			formatPdfToEmail,
			formatPdfOptions,
			formatSbomOptions,
			targetFile,
			targetPath,
			params)
	}
}

func runGetCodeBashingCommand(
	codeBashingWrapper wrappers.CodeBashingWrapper,
) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		language, _ := cmd.Flags().GetString(commonParams.LanguageFlag)
		cwe, _ := cmd.Flags().GetString(commonParams.CweIDFlag)
		vulType, _ := cmd.Flags().GetString(commonParams.VulnerabilityTypeFlag)
		params, err := codeBashingWrapper.BuildCodeBashingParams(
			[]wrappers.CodeBashingParamsCollection{
				{
					CweID:       "CWE-" + cwe,
					Language:    language,
					CxQueryName: strings.ReplaceAll(vulType, " ", "_"),
				},
			},
		)
		if err != nil {
			return err
		}
		// Fetch the cached token or a new one to obtain the codebashing URL incoded in the jwt token
		codeBashingURL, err := codeBashingWrapper.GetCodeBashingURL(codeBashingKey)
		if err != nil {
			return err
		}
		// Make the request to the api to obtain the codebashing link and send the codebashing url to enrich the path
		CodeBashingModel, webError, err := codeBashingWrapper.GetCodeBashingLinks(params, codeBashingURL)
		if err != nil {
			return err
		}
		if webError != nil {
			return errors.New(webError.Message)
		}
		err = printByFormat(cmd, *CodeBashingModel)
		if err != nil {
			return errors.Wrapf(err, "%s", failedListingCodeBashing)
		}
		return nil
	}
}

func CreateScanReport(
	resultsWrapper wrappers.ResultsWrapper,
	risksOverviewWrapper wrappers.RisksOverviewWrapper,
	resultsSbomWrapper wrappers.ResultsSbomWrapper,
	policyResponseModel *wrappers.PolicyResponseModel,
	useSCALocalFlow bool,
	retrySBOM int,
	resultsPdfReportsWrapper wrappers.ResultsPdfWrapper,
	scan *wrappers.ScanResponseModel,
	reportTypes,
	formatPdfToEmail,
	formatPdfOptions,
	formatSbomOptions,
	targetFile,
	targetPath string,
	params map[string]string,
) error {
	reportList := strings.Split(reportTypes, ",")
	results := &wrappers.ScanResultsCollection{}

	summary, err := convertScanToResultsSummary(scan, resultsWrapper)
	if err != nil {
		return err
	}

	scanPending := isScanPending(summary.Status)

	err = createDirectory(targetPath)
	if err != nil {
		return err
	}
	if !scanPending {
		results, err = ReadResults(resultsWrapper, scan, params)
		if err != nil {
			return err
		}
	}
	isSummaryNeeded := verifyFormatsByReportList(reportList, summaryFormats...)
	if isSummaryNeeded && !scanPending {
		summary, err = summaryReport(summary, policyResponseModel, risksOverviewWrapper, results)
		if err != nil {
			return err
		}
	}
	for _, reportType := range reportList {
		err = createReport(reportType, formatPdfToEmail, formatPdfOptions, formatSbomOptions, targetFile,
			targetPath, results, summary, resultsSbomWrapper, resultsPdfReportsWrapper, useSCALocalFlow, retrySBOM)
		if err != nil {
			return err
		}
	}
	return nil
}

func countResult(summary *wrappers.ResultSummary, result *wrappers.ScanResult) {
	engineType := strings.TrimSpace(result.Type)
	if contains(summary.EnginesEnabled, engineType) && isExploitable(result.State) {
		if engineType == commonParams.SastType {
			summary.SastIssues++
			summary.TotalIssues++
		} else if engineType == commonParams.ScaType {
			summary.ScaIssues++
			summary.TotalIssues++
		} else if engineType == commonParams.KicsType {
			summary.KicsIssues++
			summary.TotalIssues++
		}
		severity := strings.ToLower(result.Severity)
		if severity == highLabel {
			summary.HighIssues++
		} else if severity == lowLabel {
			summary.LowIssues++
		} else if severity == mediumLabel {
			summary.MediumIssues++
		} else if severity == infoLabel {
			summary.InfoIssues++
		}
	}
}

func verifyFormatsByReportList(reportFormats []string, formats ...string) bool {
	for _, reportFormat := range reportFormats {
		for _, format := range formats {
			if printer.IsFormat(reportFormat, format) {
				return true
			}
		}
	}
	return false
}

func validateEmails(emailString string) ([]string, error) {
	re := regexp.MustCompile(`^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$`)
	emails := strings.Split(emailString, ",")
	var validEmails []string
	for _, emailStr := range emails {
		email := strings.TrimSpace(emailStr)
		if re.MatchString(email) {
			validEmails = append(validEmails, email)
		} else {
			return nil, errors.Errorf("report not sent, invalid email address: %s", email)
		}
	}
	return validEmails, nil
}

func getResultsForAPISecScanner(
	risksOverviewWrapper wrappers.RisksOverviewWrapper,
	scanID string,
) (results *wrappers.APISecResult, err error) {
	var apiSecResultsModel *wrappers.APISecResult
	var errorModel *wrappers.WebError

	apiSecResultsModel, errorModel, err = risksOverviewWrapper.GetAllAPISecRisksByScanID(scanID)
	if err != nil {
		return nil, errors.Wrapf(err, "%s", failedListingResults)
	}
	if errorModel != nil {
		return nil, errors.Errorf("%s: CODE: %d, %s", failedListingResults, errorModel.Code, errorModel.Message)
	} else if apiSecResultsModel != nil {
		return apiSecResultsModel, nil
	}
	return nil, nil
}

func isScanPending(scanStatus string) bool {
	return !(strings.EqualFold(scanStatus, "Completed") || strings.EqualFold(
		scanStatus,
		"Partial",
	) || strings.EqualFold(scanStatus, "Failed"))
}

func isValidScanStatus(status, format string) bool {
	if isScanPending(status) {
		log.Printf("Result format file %s not create because scan status is %s", format, status)
		return false
	}
	return true
}

func createReport(format,
	formatPdfToEmail,
	formatPdfOptions,
	formatSbomOptions,
	targetFile,
	targetPath string,
	results *wrappers.ScanResultsCollection,
	summary *wrappers.ResultSummary,
	resultsSbomWrapper wrappers.ResultsSbomWrapper,
	resultsPdfReportsWrapper wrappers.ResultsPdfWrapper,
	useSCALocalFlow bool,
	retrySBOM int) error {
	if printer.IsFormat(format, printer.FormatSarif) && isValidScanStatus(summary.Status, printer.FormatSarif) {
		sarifRpt := createTargetName(targetFile, targetPath, printer.FormatSarif)
		return exportSarifResults(sarifRpt, results)
	}
	if printer.IsFormat(format, printer.FormatSonar) && isValidScanStatus(summary.Status, printer.FormatSonar) {
		sonarRpt := createTargetName(fmt.Sprintf("%s%s", targetFile, sonarTypeLabel), targetPath, printer.FormatJSON)
		return exportSonarResults(sonarRpt, results)
	}
	if printer.IsFormat(format, printer.FormatJSON) && isValidScanStatus(summary.Status, printer.FormatJSON) {
		jsonRpt := createTargetName(targetFile, targetPath, printer.FormatJSON)
		return exportJSONResults(jsonRpt, results)
	}
	if printer.IsFormat(format, printer.FormatGL) {
		jsonRpt := createTargetName(fmt.Sprintf("%s%s", targetFile, glSastTypeLobel), targetPath, printer.FormatJSON)
		return exportGlSastResults(jsonRpt, results, summary)
	}
	if printer.IsFormat(format, printer.FormatSummaryConsole) {
		return writeConsoleSummary(summary)
	}
	if printer.IsFormat(format, printer.FormatSummary) {
		summaryRpt := createTargetName(targetFile, targetPath, printer.FormatHTML)
		convertNotAvailableNumberToZero(summary)
		return writeHTMLSummary(summaryRpt, summary)
	}
	if printer.IsFormat(format, printer.FormatSummaryJSON) {
		summaryRpt := createTargetName(targetFile, targetPath, printer.FormatJSON)
		convertNotAvailableNumberToZero(summary)
		return exportJSONSummaryResults(summaryRpt, summary)
	}
	if printer.IsFormat(format, printer.FormatPDF) && isValidScanStatus(summary.Status, printer.FormatPDF) {
		summaryRpt := createTargetName(targetFile, targetPath, printer.FormatPDF)
		return exportPdfResults(resultsPdfReportsWrapper, summary, summaryRpt, formatPdfToEmail, formatPdfOptions)
	}
	if printer.IsFormat(format, printer.FormatSummaryMarkdown) {
		summaryRpt := createTargetName(targetFile, targetPath, "md")
		convertNotAvailableNumberToZero(summary)
		return writeMarkdownSummary(summaryRpt, summary)
	}
	if printer.IsFormat(format, printer.FormatSbom) && isValidScanStatus(summary.Status, printer.FormatSbom) {
		targetType := printer.FormatJSON
		if strings.Contains(strings.ToLower(formatSbomOptions), printer.FormatXML) {
			targetType = printer.FormatXML
		}
		summaryRpt := createTargetName(fmt.Sprintf("%s_%s", targetFile, printer.FormatSbom), targetPath, targetType)
		convertNotAvailableNumberToZero(summary)

		if !contains(summary.EnginesEnabled, commonParams.ScaType) {
			return fmt.Errorf("unable to generate %s report - SCA engine must be enabled on scan summary", printer.FormatSbom)
		}

		if summary.ScaIssues == notAvailableNumber {
			return fmt.Errorf("unable to generate %s report - SCA engine did not complete successfully", printer.FormatSbom)
		}

		return exportSbomResults(resultsSbomWrapper, summaryRpt, summary, formatSbomOptions, useSCALocalFlow, retrySBOM)
	}
	return fmt.Errorf("bad report format %s", format)
}

func createTargetName(targetFile, targetPath, targetType string) string {
	return filepath.Join(targetPath, targetFile+"."+targetType)
}

func createDirectory(targetPath string) error {
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		log.Printf("\nOutput path not found: %s\n", targetPath)
		log.Printf("Creating directory: %s\n", targetPath)
		err = os.Mkdir(targetPath, directoryPermission)
		if err != nil {
			return err
		}
	}
	return nil
}

func ReadResults(
	resultsWrapper wrappers.ResultsWrapper,
	scan *wrappers.ScanResponseModel,
	params map[string]string,
) (results *wrappers.ScanResultsCollection, err error) {
	var resultsModel *wrappers.ScanResultsCollection
	var errorModel *wrappers.WebError

	params[commonParams.ScanIDQueryParam] = scan.ID
	resultsModel, errorModel, err = resultsWrapper.GetAllResultsByScanID(params)

	if err != nil {
		return nil, errors.Wrapf(err, "%s", failedListingResults)
	}
	if errorModel != nil {
		return nil, errors.Errorf("%s: CODE: %d, %s", failedListingResults, errorModel.Code, errorModel.Message)
	}

	if resultsModel != nil {
		resultsModel, err = enrichScaResults(resultsWrapper, scan, params, resultsModel)
		if err != nil {
			return nil, err
		}

		resultsModel.ScanID = scan.ID
		return resultsModel, nil
	}
	return nil, nil
}

func enrichScaResults(
	resultsWrapper wrappers.ResultsWrapper,
	scan *wrappers.ScanResponseModel,
	params map[string]string,
	resultsModel *wrappers.ScanResultsCollection,
) (*wrappers.ScanResultsCollection, error) {
	if util.Contains(scan.Engines, commonParams.ScaType) {
		// Get additional information to enrich sca results
		scaPackageModel, errorModel, err := resultsWrapper.GetAllResultsPackageByScanID(params)
		if errorModel != nil {
			return nil, errors.Errorf("%s: CODE: %d, %s", failedListingResults, errorModel.Code, errorModel.Message)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "%s", failedListingResults)
		}
		// Get additional information to add the type information to the sca results
		scaTypeModel, errorModel, err := resultsWrapper.GetAllResultsTypeByScanID(params)
		if errorModel != nil {
			return nil, errors.Errorf("%s: CODE: %d, %s", failedListingResults, errorModel.Code, errorModel.Message)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "%s", failedListingResults)
		}
		// Enrich sca results
		if scaPackageModel != nil {
			resultsModel = addPackageInformation(resultsModel, scaPackageModel, scaTypeModel)
		}
	}
	_, sastRedundancy := params[commonParams.SastRedundancyFlag]

	if util.Contains(scan.Engines, commonParams.SastType) && sastRedundancy {
		// Compute SAST results redundancy
		resultsModel = ComputeRedundantSastResults(resultsModel)
	}
	return resultsModel, nil
}

func exportSarifResults(targetFile string, results *wrappers.ScanResultsCollection) error {
	var err error
	var resultsJSON []byte
	log.Println("Creating SARIF Report: ", targetFile)
	var sarifResults = convertCxResultsToSarif(results)
	resultsJSON, err = json.Marshal(sarifResults)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to serialize results response ", failedGettingAll)
	}
	f, err := os.Create(targetFile)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to create target file  ", failedGettingAll)
	}
	_, _ = fmt.Fprintln(f, string(resultsJSON))
	_ = f.Close()
	return nil
}
func exportGlSastResults(targetFile string, results *wrappers.ScanResultsCollection, summary *wrappers.ResultSummary) error {
	log.Println("Creating gl-sast Report: ", targetFile)
	var glSast = new(wrappers.GlSastResultsCollection)
	err := addScanToGlSastReport(summary, glSast)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to add scan to gl sast report", failedListingResults)
	}
	convertCxResultToGlVulnerability(results, glSast, summary.BaseURI)
	resultsJSON, err := json.Marshal(glSast)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to serialize gl sast report ", failedListingResults)
	}
	f, err := os.Create(targetFile)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to create target file  ", failedListingResults)
	}
	_, _ = fmt.Fprintln(f, string(resultsJSON))
	defer f.Close()
	return nil
}
func addScanToGlSastReport(summary *wrappers.ResultSummary, glSast *wrappers.GlSastResultsCollection) error {
	createdAt, err := time.Parse(summaryCreatedAtLayout, summary.CreatedAt)
	if err != nil {
		return err
	}

	glSast.Scan = wrappers.ScanGlReport{}
	glSast.Schema = "https://gitlab.com/gitlab-org/gitlab/-/raw/master/lib/gitlab/ci/parsers/security/validators/schemas/15.0.0/sast-report-format.json"
	glSast.Version = "15.0.0"
	glSast.Scan.Analyzer.URL = wrappers.AnalyzerURL
	glSast.Scan.Analyzer.Name = wrappers.VendorName
	glSast.Scan.Analyzer.Vendor.Name = wrappers.VendorName
	glSast.Scan.Analyzer.ID = wrappers.AnalyzerID
	glSast.Scan.Scanner.ID = wrappers.AnalyzerID
	glSast.Scan.Scanner.Name = wrappers.VendorName
	glSast.Scan.Status = commonParams.Success
	glSast.Scan.Type = commonParams.SastType
	glSast.Scan.StartTime = createdAt.Format(glTimeFormat)
	glSast.Scan.EndTime = createdAt.Format(glTimeFormat)
	glSast.Scan.Scanner.Vendor.Name = wrappers.VendorName
	glSast.Scan.Scanner.Version = commonParams.Version
	glSast.Scan.Analyzer.Version = commonParams.Version

	return nil
}
func exportSonarResults(targetFile string, results *wrappers.ScanResultsCollection) error {
	var err error
	var resultsJSON []byte
	log.Println("Creating SONAR Report: ", targetFile)
	var sonarResults = convertCxResultsToSonar(results)
	resultsJSON, err = json.Marshal(sonarResults)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to serialize results response ", failedGettingAll)
	}
	f, err := os.Create(targetFile)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to create target file  ", failedGettingAll)
	}
	_, _ = fmt.Fprintln(f, string(resultsJSON))
	_ = f.Close()
	return nil
}
func exportJSONResults(targetFile string, results *wrappers.ScanResultsCollection) error {
	var err error
	var resultsJSON []byte
	log.Println("Creating JSON Report: ", targetFile)
	resultsJSON, err = json.Marshal(results)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to serialize results response ", failedGettingAll)
	}
	f, err := os.Create(targetFile)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to create target file  ", failedGettingAll)
	}
	_, _ = fmt.Fprintln(f, string(resultsJSON))
	_ = f.Close()
	return nil
}

func exportJSONSummaryResults(targetFile string, results *wrappers.ResultSummary) error {
	var err error
	var resultsJSON []byte
	log.Println("Creating summary JSON Report: ", targetFile)
	resultsJSON, err = json.Marshal(results)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to serialize results response ", failedGettingAll)
	}
	f, err := os.Create(targetFile)
	if err != nil {
		return errors.Wrapf(err, "%s: failed to create target file  ", failedGettingAll)
	}
	_, _ = fmt.Fprintln(f, string(resultsJSON))
	_ = f.Close()
	return nil
}

func exportSbomResults(sbomWrapper wrappers.ResultsSbomWrapper,
	targetFile string,
	results *wrappers.ResultSummary,
	formatSbomOptions string,
	useSCALocalFlow bool,
	retrySBOM int) error {
	payload := &wrappers.SbomReportsPayload{
		ScanID:     results.ScanID,
		FileFormat: defaultSbomOption,
	}
	if formatSbomOptions != "" && formatSbomOptions != defaultSbomOption {
		format, err := validateSbomOptions(formatSbomOptions)
		if err != nil {
			return err
		}
		payload.FileFormat = format
	}
	if useSCALocalFlow {
		pollingResp := &wrappers.SbomPollingResponse{}

		sbomresp, err := sbomWrapper.GenerateSbomReport(payload)
		if err != nil {
			return err
		}

		log.Println("Generating SBOM report with " + payload.FileFormat + " file format")
		pollingResp.ExportStatus = exportingStatus
		for pollingResp.ExportStatus == exportingStatus || pollingResp.ExportStatus == pendingStatus {
			pollingResp, err = sbomWrapper.GetSbomReportStatus(sbomresp.ExportID)
			if err != nil {
				return errors.Wrapf(err, "%s", "failed getting SBOM report status")
			}
			time.Sleep(delayValueForReport * time.Second)
		}
		if !strings.EqualFold(pollingResp.ExportStatus, completedStatus) {
			return errors.Errorf("SBOM generating failed - Current status: %s", pollingResp.ExportStatus)
		}
		err = sbomWrapper.DownloadSbomReport(pollingResp.ExportID, targetFile)
		if err != nil {
			return errors.Wrapf(err, "%s", "Failed downloading SBOM report")
		}
		return nil
	}
	log.Println("Generating SBOM report with " + payload.FileFormat + " file format using SCA proxy...")

	i := 0
	for i < retrySBOM {
		completed, err := sbomWrapper.GenerateSbomReportWithProxy(payload, targetFile)
		if err != nil {
			return err
		}
		if completed {
			return nil
		}
		i++
		time.Sleep(delayValueForReport * time.Second)
		logger.PrintIfVerbose(
			fmt.Sprintf(
				"Retry SBOM report: %d retry",
				i,
			),
		)
	}
	return nil
}
func exportPdfResults(pdfWrapper wrappers.ResultsPdfWrapper, summary *wrappers.ResultSummary, summaryRpt, formatPdfToEmail, pdfOptions string) error {
	pdfReportsPayload := &wrappers.PdfReportsPayload{}
	pollingResp := &wrappers.PdfPollingResponse{}
	pdfOptionsSections, pdfOptionsEngines, err := parsePDFOptions(pdfOptions, summary.EnginesEnabled)
	if err != nil {
		return err
	}
	pdfReportsPayload.ReportName = reportNameScanReport
	pdfReportsPayload.ReportType = "cli"
	pdfReportsPayload.FileFormat = printer.FormatPDF
	pdfReportsPayload.Data.ScanID = summary.ScanID
	pdfReportsPayload.Data.ProjectID = summary.ProjectID
	pdfReportsPayload.Data.BranchName = summary.BranchName
	pdfReportsPayload.Data.Scanners = pdfOptionsEngines
	pdfReportsPayload.Data.Sections = pdfOptionsSections

	// will generate pdf report and send it to the email list
	// instead of saving it to the file system
	if len(formatPdfToEmail) > 0 {
		emailList, validateErr := validateEmails(formatPdfToEmail)
		if validateErr != nil {
			return validateErr
		}
		pdfReportsPayload.ReportType = reportTypeEmail
		pdfReportsPayload.Data.Email = emailList
	}
	pdfReportID, webErr, err := pdfWrapper.GeneratePdfReport(pdfReportsPayload)
	if webErr != nil {
		return errors.Errorf("Error generating PDF report - %s", webErr.Message)
	}
	if err != nil {
		return errors.Errorf("Error generating PDF report - %s", err.Error())
	}
	if pdfReportsPayload.ReportType == reportTypeEmail {
		log.Println("Sending PDF report to: ", pdfReportsPayload.Data.Email)
		return nil
	}
	log.Println("Generating PDF report")
	pollingResp.Status = startedStatus
	for pollingResp.Status == startedStatus || pollingResp.Status == requestedStatus {
		pollingResp, webErr, err = pdfWrapper.CheckPdfReportStatus(pdfReportID.ReportID)
		if err != nil || webErr != nil {
			return errors.Wrapf(err, "%v", webErr)
		}
		logger.PrintfIfVerbose("PDF report status: %s", pollingResp.Status)
		time.Sleep(delayValueForReport * time.Millisecond)
	}
	if pollingResp.Status != completedStatus {
		return errors.Errorf("PDF generating failed - Current status: %s", pollingResp.Status)
	}
	err = pdfWrapper.DownloadPdfReport(pdfReportID.ReportID, summaryRpt)
	if err != nil {
		return errors.Wrapf(err, "%s", "Failed downloading PDF report")
	}
	return nil
}

func validateSbomOptions(sbomOption string) (string, error) {
	var sbomOptionsStringMap = map[string]string{
		"cyclonedxjson": "CycloneDxJson",
		"cyclonedxxml":  "CycloneDxXml",
		"spdxjson":      "SpdxJson",
	}
	sbomOption = strings.ToLower(strings.ReplaceAll(sbomOption, " ", ""))
	if sbomOptionsStringMap[sbomOption] != "" {
		return sbomOptionsStringMap[sbomOption], nil
	}
	return "", errors.Errorf("invalid SBOM option: %s", sbomOption)
}

func parsePDFOptions(pdfOptions string, enabledEngines []string) (pdfOptionsSections, pdfOptionsEngines []string, err error) {
	var pdfOptionsSectionsMap = map[string]string{
		"scansummary":      "ScanSummary",
		"executivesummary": "ExecutiveSummary",
		"scanresults":      "ScanResults",
	}
	var pdfOptionsEnginesMap = map[string]string{
		commonParams.ScaType:  "SCA",
		commonParams.SastType: "SAST",
		commonParams.KicsType: "KICS",
		commonParams.IacType:  "KICS",
	}
	pdfOptions = strings.ToLower(strings.ReplaceAll(pdfOptions, " ", ""))
	options := strings.Split(strings.ReplaceAll(pdfOptions, "\n", ""), ",")
	for _, s := range options {
		if pdfOptionsEnginesMap[s] != "" {
			pdfOptionsEngines = append(pdfOptionsEngines, pdfOptionsEnginesMap[s])
		} else if pdfOptionsSectionsMap[s] != "" {
			pdfOptionsSections = append(pdfOptionsSections, pdfOptionsSectionsMap[s])
		} else {
			return nil, nil, errors.Errorf("report option \"%s\" unavailable", s)
		}
	}
	if pdfOptionsEngines == nil {
		for _, engine := range enabledEngines {
			if pdfOptionsEnginesMap[engine] != "" {
				pdfOptionsEngines = append(pdfOptionsEngines, pdfOptionsEnginesMap[engine])
			}
		}
	}
	return pdfOptionsSections, pdfOptionsEngines, nil
}

func convertCxResultsToSarif(results *wrappers.ScanResultsCollection) *wrappers.SarifResultsCollection {
	var sarif = new(wrappers.SarifResultsCollection)
	sarif.Schema = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json"
	sarif.Version = "2.1.0"
	sarif.Runs = []wrappers.SarifRun{}
	sarif.Runs = append(sarif.Runs, createSarifRun(results))
	return sarif
}

func convertCxResultToGlVulnerability(results *wrappers.ScanResultsCollection, glSast *wrappers.GlSastResultsCollection, summaryBaseURI string) {
	for _, result := range results.Results {
		if strings.TrimSpace(result.Type) == commonParams.SastType {
			glSast = parseGlSastVulnerability(result, glSast, summaryBaseURI)
		}
	}
}

func parseGlSastVulnerability(result *wrappers.ScanResult, glSast *wrappers.GlSastResultsCollection, summaryBaseURI string) *wrappers.GlSastResultsCollection {
	queryName := result.ScanResultData.QueryName
	fileName := result.ScanResultData.Nodes[0].FileName
	lineNumber := strconv.FormatUint(uint64(result.ScanResultData.Nodes[0].Line), 10)
	startLine := result.ScanResultData.Nodes[0].Line
	endLine := result.ScanResultData.Nodes[0].Line + result.ScanResultData.Nodes[0].Length
	ID := fmt.Sprintf("%s:%s:%s", queryName, fileName, lineNumber)
	category := fmt.Sprintf("%s-%s", wrappers.VendorName, result.Type)
	message := fmt.Sprintf("%s@%s:%s", queryName, fileName, lineNumber)

	glSast.Vulnerabilities = append(glSast.Vulnerabilities, wrappers.GlVulnerabilities{
		ID:          ID,
		Category:    category,
		Name:        queryName,
		Message:     message,
		Description: result.Description,
		CVE:         ID,
		Severity:    cases.Title(language.English).String(result.Severity),
		Confidence:  cases.Title(language.English).String(result.Severity),
		Solution:    "",

		Scanner: wrappers.GlScanner{
			ID:   category,
			Name: category,
		},
		Identifiers: []wrappers.Identifier{
			{
				Type:  "cxOneScan",
				Name:  "CxOne Scan",
				URL:   summaryBaseURI,
				Value: result.ID,
			},
		},
		Links: make([]string, 0),
		Tracking: wrappers.Tracking{
			Type: "source",
			Items: []wrappers.Item{
				{
					Signatures: []wrappers.Signature{{Algorithm: result.Type + "-Algorithm ", Value: "NA"}},
					File:       fileName,
					EndLine:    endLine,
					StartLine:  startLine,
				},
			},
		},
		Flags: make([]wrappers.Flag, 0),
		Location: wrappers.Location{
			File:      fileName,
			StartLine: startLine,
			EndLine:   endLine,
		},
	})
	return glSast
}

func convertCxResultsToSonar(results *wrappers.ScanResultsCollection) *wrappers.ScanResultsSonar {
	var sonar = new(wrappers.ScanResultsSonar)
	sonar.Results = parseResultsSonar(results)
	return sonar
}

func createSarifRun(results *wrappers.ScanResultsCollection) wrappers.SarifRun {
	var sarifRun wrappers.SarifRun
	sarifRun.Tool.Driver.Name = wrappers.SarifName
	sarifRun.Tool.Driver.Version = wrappers.SarifVersion
	sarifRun.Tool.Driver.InformationURI = wrappers.SarifInformationURI
	sarifRun.Tool.Driver.Rules, sarifRun.Results = parseResults(results)
	return sarifRun
}

func parseResults(results *wrappers.ScanResultsCollection) ([]wrappers.SarifDriverRule, []wrappers.SarifScanResult) {
	var sarifRules = make([]wrappers.SarifDriverRule, 0)
	var sarifResults = make([]wrappers.SarifScanResult, 0)
	if results != nil {
		ruleIds := map[interface{}]bool{}
		for _, result := range results.Results {
			if rule := findRule(ruleIds, result); rule != nil {
				sarifRules = append(sarifRules, *rule)
			}
			if sarifResult := findResult(result); sarifResult != nil {
				sarifResults = append(sarifResults, sarifResult...)
			}
		}
	}
	return sarifRules, sarifResults
}

func parseResultsSonar(results *wrappers.ScanResultsCollection) []wrappers.SonarIssues {
	var sonarIssues []wrappers.SonarIssues

	if results != nil {
		for _, result := range results.Results {
			var auxIssue = initSonarIssue(result)

			engineType := strings.TrimSpace(result.Type)

			if engineType == commonParams.SastType {
				auxIssue.PrimaryLocation = parseSonarPrimaryLocation(result)
				auxIssue.SecondaryLocations = parseSonarSecondaryLocations(result)
				sonarIssues = append(sonarIssues, auxIssue)
			} else if engineType == commonParams.KicsType {
				auxIssue.PrimaryLocation = parseLocationKics(result)
				sonarIssues = append(sonarIssues, auxIssue)
			} else if engineType == commonParams.ScaType {
				sonarIssuesByLocation := parseScaSonarLocations(result)
				sonarIssues = append(sonarIssues, sonarIssuesByLocation...)
			}
		}
	}
	return sonarIssues
}

func initSonarIssue(result *wrappers.ScanResult) wrappers.SonarIssues {
	var sonarIssue wrappers.SonarIssues
	sonarIssue.Severity = sonarSeverities[result.Severity]
	sonarIssue.Type = vulnerabilitySonar
	sonarIssue.EngineID = result.Type
	sonarIssue.RuleID = result.ID
	sonarIssue.EffortMinutes = 0

	return sonarIssue
}

func parseScaSonarLocations(result *wrappers.ScanResult) []wrappers.SonarIssues {
	if result == nil || result.ScanResultData.ScaPackageCollection == nil || result.ScanResultData.ScaPackageCollection.Locations == nil {
		return []wrappers.SonarIssues{}
	}

	var issuesByLocation []wrappers.SonarIssues

	for _, location := range result.ScanResultData.ScaPackageCollection.Locations {
		issueByLocation := initSonarIssue(result)

		var primaryLocation wrappers.SonarLocation

		primaryLocation.FilePath = *location
		_, _, primaryLocation.Message = findRuleID(result)

		var textRange wrappers.SonarTextRange
		textRange.StartColumn = 1
		textRange.EndColumn = 2
		textRange.StartLine = 1
		textRange.EndLine = 2

		primaryLocation.TextRange = textRange

		issueByLocation.PrimaryLocation = primaryLocation

		issuesByLocation = append(issuesByLocation, issueByLocation)
	}

	return issuesByLocation
}

func parseLocationKics(results *wrappers.ScanResult) wrappers.SonarLocation {
	var auxLocation wrappers.SonarLocation
	auxLocation.FilePath = strings.TrimLeft(results.ScanResultData.Filename, "/")
	auxLocation.Message = results.ScanResultData.Value
	var auxTextRange wrappers.SonarTextRange
	auxTextRange.StartLine = results.ScanResultData.Line
	auxTextRange.StartColumn = 0
	auxTextRange.EndColumn = 1
	auxLocation.TextRange = auxTextRange
	return auxLocation
}

func parseSonarPrimaryLocation(results *wrappers.ScanResult) wrappers.SonarLocation {
	var auxLocation wrappers.SonarLocation
	// fill the details in the primary Location
	if len(results.ScanResultData.Nodes) > 0 {
		auxLocation.FilePath = strings.TrimLeft(results.ScanResultData.Nodes[0].FileName, "/")
		auxLocation.Message = strings.ReplaceAll(results.ScanResultData.QueryName, "_", " ")
		auxLocation.TextRange = parseSonarTextRange(results.ScanResultData.Nodes[0])
	}
	return auxLocation
}

func parseSonarSecondaryLocations(results *wrappers.ScanResult) []wrappers.SonarLocation {
	var auxSecondaryLocations []wrappers.SonarLocation
	// Traverse all the rest of the scan result nodes into secondary location of sonar
	if len(results.ScanResultData.Nodes) >= 1 {
		for _, node := range results.ScanResultData.Nodes[1:] {
			var auxSecondaryLocation wrappers.SonarLocation
			auxSecondaryLocation.FilePath = strings.TrimLeft(node.FileName, "/")
			auxSecondaryLocation.Message = strings.ReplaceAll(results.ScanResultData.QueryName, "_", " ")
			auxSecondaryLocation.TextRange = parseSonarTextRange(node)
			auxSecondaryLocations = append(auxSecondaryLocations, auxSecondaryLocation)
		}
	}
	return auxSecondaryLocations
}

func parseSonarTextRange(results *wrappers.ScanResultNode) wrappers.SonarTextRange {
	var auxTextRange wrappers.SonarTextRange
	auxTextRange.StartLine = results.Line
	auxTextRange.StartColumn = results.Column
	auxTextRange.EndColumn = results.Column + results.Length
	if auxTextRange.StartColumn == auxTextRange.EndColumn {
		auxTextRange.EndColumn++
	}
	return auxTextRange
}

func findRule(ruleIds map[interface{}]bool, result *wrappers.ScanResult) *wrappers.SarifDriverRule {
	var sarifRule wrappers.SarifDriverRule
	sarifRule.ID, sarifRule.Name, _ = findRuleID(result)
	sarifRule.FullDescription = findFullDescription(result)
	sarifRule.Help = findHelp(result)
	sarifRule.HelpURI = wrappers.SarifInformationURI
	sarifRule.Properties = findProperties(result)

	if !ruleIds[sarifRule.ID] {
		ruleIds[sarifRule.ID] = true
		return &sarifRule
	}

	return nil
}

func findRuleID(result *wrappers.ScanResult) (ruleID, ruleName, shortMessage string) {
	if result.ScanResultData.QueryID == nil {
		return fmt.Sprintf("%s (%s)", result.ID, result.Type),
			strings.Title(strings.ToLower(strings.ReplaceAll(result.ID, "-", ""))),
			fmt.Sprintf("%s (%s)", result.ScanResultData.PackageIdentifier, result.ID)
	}

	return fmt.Sprintf("%v (%s)", result.ScanResultData.QueryID, result.Type),
		strings.ReplaceAll(result.ScanResultData.QueryName, "_", " "),
		strings.ReplaceAll(result.ScanResultData.QueryName, "_", " ")
}

func findFullDescription(result *wrappers.ScanResult) wrappers.SarifDescription {
	var sarifDescription wrappers.SarifDescription
	sarifDescription.Text = findDescriptionText(result)
	return sarifDescription
}

func findHelp(result *wrappers.ScanResult) wrappers.SarifHelp {
	var sarifHelp wrappers.SarifHelp
	sarifHelp.Text = findDescriptionText(result)
	sarifHelp.Markdown = findHelpMarkdownText(result)

	return sarifHelp
}

func findDescriptionText(result *wrappers.ScanResult) string {
	if result.Type == commonParams.KicsType {
		return fmt.Sprintf(
			"%s Value: %s Excepted value: %s",
			result.Description, result.ScanResultData.Value, result.ScanResultData.ExpectedValue,
		)
	}

	return result.Description
}

func findHelpMarkdownText(result *wrappers.ScanResult) string {
	if result.Type == commonParams.KicsType {
		return fmt.Sprintf(
			"%s <br><br><strong>Value:</strong> %s <br><strong>Excepted value:</strong> %s",
			result.Description, result.ScanResultData.Value, result.ScanResultData.ExpectedValue,
		)
	}

	return result.Description
}

func findProperties(result *wrappers.ScanResult) wrappers.SarifProperties {
	var sarifProperties wrappers.SarifProperties
	sarifProperties.ID, sarifProperties.Name, _ = findRuleID(result)
	sarifProperties.Description = findDescriptionText(result)
	sarifProperties.SecuritySeverity = securities[result.Severity]
	sarifProperties.Tags = []string{"security", "checkmarx", result.Type}

	return sarifProperties
}

func findSarifLevel(result *wrappers.ScanResult) string {
	level := map[string]string{
		infoCx:   infoLowSarif,
		lowCx:    infoLowSarif,
		mediumCx: mediumSarif,
		highCx:   highSarif,
	}
	return level[result.Severity]
}

func initSarifResult(result *wrappers.ScanResult) wrappers.SarifScanResult {
	var scanResult wrappers.SarifScanResult
	scanResult.RuleID, _, scanResult.Message.Text = findRuleID(result)
	scanResult.Level = findSarifLevel(result)
	scanResult.Locations = []wrappers.SarifLocation{}

	return scanResult
}

func findResult(result *wrappers.ScanResult) []wrappers.SarifScanResult {
	var scanResults []wrappers.SarifScanResult

	if len(result.ScanResultData.Nodes) > 0 {
		scanResults = parseSarifResultSast(result, scanResults)
	} else if result.Type == commonParams.KicsType {
		scanResults = parseSarifResultKics(result, scanResults)
	} else if result.Type == commonParams.ScaType {
		scanResults = parseSarifResultsSca(result, scanResults)
	}

	if len(scanResults) > 0 {
		return scanResults
	}
	return nil
}

func parseSarifResultsSca(result *wrappers.ScanResult, scanResults []wrappers.SarifScanResult) []wrappers.SarifScanResult {
	if result == nil || result.ScanResultData.ScaPackageCollection == nil || result.ScanResultData.ScaPackageCollection.Locations == nil {
		return scanResults
	}
	for _, location := range result.ScanResultData.ScaPackageCollection.Locations {
		var scanResult = initSarifResult(result)

		var scanLocation wrappers.SarifLocation
		scanLocation.PhysicalLocation.ArtifactLocation.URI = *location
		scanLocation.PhysicalLocation.Region = &wrappers.SarifRegion{}
		scanLocation.PhysicalLocation.Region.StartLine = 1
		scanLocation.PhysicalLocation.Region.StartColumn = 1
		scanLocation.PhysicalLocation.Region.EndColumn = 2
		scanResult.Locations = append(scanResult.Locations, scanLocation)

		scanResults = append(scanResults, scanResult)
	}
	return scanResults
}

func parseSarifResultKics(result *wrappers.ScanResult, scanResults []wrappers.SarifScanResult) []wrappers.SarifScanResult {
	var scanResult = initSarifResult(result)
	var scanLocation wrappers.SarifLocation

	scanLocation.PhysicalLocation.ArtifactLocation.URI = strings.Replace(
		result.ScanResultData.Filename,
		"/",
		"",
		1,
	)
	scanLocation.PhysicalLocation.Region = &wrappers.SarifRegion{}
	scanLocation.PhysicalLocation.Region.StartLine = result.ScanResultData.Line
	scanLocation.PhysicalLocation.Region.StartColumn = 1
	scanLocation.PhysicalLocation.Region.EndColumn = 2
	scanResult.Locations = append(scanResult.Locations, scanLocation)

	scanResults = append(scanResults, scanResult)
	return scanResults
}

func parseSarifResultSast(result *wrappers.ScanResult, scanResults []wrappers.SarifScanResult) []wrappers.SarifScanResult {
	if result == nil || result.ScanResultData.Nodes == nil {
		return scanResults
	}
	var scanResult = initSarifResult(result)

	for _, node := range result.ScanResultData.Nodes {
		var scanLocation wrappers.SarifLocation
		if len(node.FileName) >= sarifNodeFileLength {
			scanLocation.PhysicalLocation.ArtifactLocation.URI = node.FileName[1:]
			if node.Line <= 0 {
				continue
			}
			scanLocation.PhysicalLocation.Region = &wrappers.SarifRegion{}
			scanLocation.PhysicalLocation.Region.StartLine = node.Line
			column := node.Column
			length := node.Length
			scanLocation.PhysicalLocation.Region.StartColumn = column
			scanLocation.PhysicalLocation.Region.EndColumn = column + length

			scanResult.Locations = append(scanResult.Locations, scanLocation)
		}
	}

	scanResults = append(scanResults, scanResult)
	return scanResults
}

func convertNotAvailableNumberToZero(summary *wrappers.ResultSummary) {
	if summary.KicsIssues == notAvailableNumber {
		summary.KicsIssues = 0
	} else if summary.SastIssues == notAvailableNumber {
		summary.SastIssues = 0
	} else if summary.ScaIssues == notAvailableNumber {
		summary.ScaIssues = 0
	}
}

func buildAuxiliaryScaMaps(resultsModel *wrappers.ScanResultsCollection, scaPackageModel *[]wrappers.ScaPackageCollection,
	scaTypeModel *[]wrappers.ScaTypeCollection) (locationsByID map[string][]*string, typesByCVE map[string]wrappers.ScaTypeCollection) {
	locationsByID = make(map[string][]*string)
	typesByCVE = make(map[string]wrappers.ScaTypeCollection)
	// Create map to be used to populate locations for each package path
	for _, result := range resultsModel.Results {
		if result.Type == commonParams.ScaType {
			for _, packages := range *scaPackageModel {
				currentPackage := packages
				locationsByID[packages.ID] = currentPackage.Locations
			}
			for _, types := range *scaTypeModel {
				typesByCVE[types.ID] = types
			}
		}
	}
	return locationsByID, typesByCVE
}

func buildScaType(typesByCVE map[string]wrappers.ScaTypeCollection, result *wrappers.ScanResult) string {
	types, ok := typesByCVE[result.ID]
	if ok && types.Type == "SupplyChain" {
		return "Supply Chain"
	}
	return "Vulnerability"
}

func buildScaState(typesByCVE map[string]wrappers.ScaTypeCollection, result *wrappers.ScanResult) string {
	types, ok := typesByCVE[result.ID]
	if ok && types.IsIgnored {
		return notExploitable
	}
	return result.State
}

func addPackageInformation(
	resultsModel *wrappers.ScanResultsCollection,
	scaPackageModel *[]wrappers.ScaPackageCollection,
	scaTypeModel *[]wrappers.ScaTypeCollection,
) *wrappers.ScanResultsCollection {
	var currentID string
	locationsByID, typesByCVE := buildAuxiliaryScaMaps(resultsModel, scaPackageModel, scaTypeModel)

	for _, result := range resultsModel.Results {
		if !(result.Type == commonParams.ScaType) {
			continue
		} else {
			currentID = result.ScanResultData.PackageIdentifier
			const precision = 1
			var roundedScore = util.RoundFloat(result.VulnerabilityDetails.CvssScore, precision)
			result.VulnerabilityDetails.CvssScore = roundedScore
			// Add the sca type
			result.ScaType = buildScaType(typesByCVE, result)
			// Temporary code for client
			result.State = buildScaState(typesByCVE, result)
			for _, packages := range *scaPackageModel {
				currentPackage := packages
				if packages.ID == currentID {
					for _, dependencyPath := range currentPackage.DependencyPathArray {
						head := &dependencyPath[0]
						head.Locations = locationsByID[head.ID]
						head.SupportsQuickFix = len(dependencyPath) == 1
						for _, location := range locationsByID[head.ID] {
							head.SupportsQuickFix = head.SupportsQuickFix && util.IsPackageFileSupported(*location)
						}
						currentPackage.SupportsQuickFix = currentPackage.SupportsQuickFix || head.SupportsQuickFix
					}
					if result.VulnerabilityDetails.CveName != "" {
						currentPackage.FixLink = "https://devhub.checkmarx.com/cve-details/" + result.VulnerabilityDetails.CveName
					} else {
						currentPackage.FixLink = ""
					}
					if currentPackage.IsDirectDependency {
						currentPackage.TypeOfDependency = directDependencyType
					} else {
						currentPackage.TypeOfDependency = indirectDependencyType
					}
					result.ScanResultData.ScaPackageCollection = &currentPackage
					break
				}
			}
		}
	}
	return resultsModel
}

func filterViolatedRules(policyModel wrappers.PolicyResponseModel) *wrappers.PolicyResponseModel {
	i := 0
	for _, policy := range policyModel.Polices {
		if len(policy.RulesViolated) > 0 {
			policyModel.Polices[i] = policy
			i++
		}
	}
	policyModel.Polices = policyModel.Polices[:i]
	return &policyModel
}
