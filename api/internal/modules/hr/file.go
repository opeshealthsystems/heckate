package hr

import (
	"net/http"
	"strings"
	"time"

	"github.com/opeshealthsystems/heckate/api/internal/platform"
	"github.com/opeshealthsystems/heckate/api/internal/reports"
)

func registerFile(a *platform.App) {
	a.Route("GET /v1/employees/{id}/file", staff, employeeFile)
	a.Route("PUT /v1/employees/{id}/private", hrRoles, putPrivate)
	a.Route("PATCH /v1/employees/{id}", hrRoles, updateEmployee)
	a.Route("POST /v1/employees/{id}/contracts", hrRoles, platform.Idempotent(createContract))
	a.Route("POST /v1/employment-contracts/{id}/activate", hrRoles, platform.Idempotent(activateContract))
	a.Route("POST /v1/employment-contracts/{id}/end", hrRoles, platform.Idempotent(endContract))
	a.Route("GET /v1/exports/employees", reviewers, exportEmployees)
}

type Private struct {
	DateOfBirth    *time.Time `json:"date_of_birth"`
	NationalID     string     `json:"national_id"`
	CNPSNumber     string     `json:"cnps_number"`
	Phone          string     `json:"phone"`
	PersonalEmail  string     `json:"personal_email"`
	Address        string     `json:"address"`
	EmergencyName  string     `json:"emergency_name"`
	EmergencyPhone string     `json:"emergency_phone"`
	BankName       string     `json:"bank_name"`
	BankAccount    string     `json:"bank_account"`
	UpdatedBy      string     `json:"updated_by"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type EmpContract struct {
	ID           string     `json:"id"`
	Type         string     `json:"type"`
	Position     string     `json:"position"`
	StartDate    time.Time  `json:"start_date"`
	EndDate      *time.Time `json:"end_date"`
	ProbationEnd *time.Time `json:"probation_end"`
	NoticeDays   int        `json:"notice_days"`
	WeeklyHours  float64    `json:"weekly_hours"`
	Status       string     `json:"status"`
	SignedOn     *time.Time `json:"signed_on"`
	DocumentRef  *string    `json:"document_ref"`
	Note         string     `json:"note"`
}

const ecCols = `id::text, type, position, start_date, end_date, probation_end, notice_days, weekly_hours::float8, status, signed_on, document_ref, note`

type Pay struct {
	EffectiveFrom time.Time `json:"effective_from"`
	BaseSalary    int64     `json:"base_salary"`
	Allowances    int64     `json:"allowances"`
}

// employeeFile gathers everything about one person that the caller may see. Rows the database hides from the
// caller (private details, pay) simply come back empty.
func employeeFile(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	e, err := scanEmp(s.Tx.QueryRow(ctx, `SELECT `+empCols+` FROM hr.employees WHERE id::text=$1`, r.PathValue("id")))
	if err != nil {
		return nil, err
	}
	body := map[string]any{"employee": e}
	var p Private
	err = s.Tx.QueryRow(ctx, `SELECT date_of_birth, national_id, cnps_number, phone, personal_email, address, emergency_name, emergency_phone, bank_name, bank_account, updated_by, updated_at
		FROM hr.employee_private WHERE employee_id=$1`, e.ID).Scan(&p.DateOfBirth, &p.NationalID, &p.CNPSNumber, &p.Phone, &p.PersonalEmail, &p.Address, &p.EmergencyName, &p.EmergencyPhone, &p.BankName, &p.BankAccount, &p.UpdatedBy, &p.UpdatedAt)
	switch {
	case err == nil:
		body["private"] = p
	case !isNF(err):
		return nil, err
	}
	rows, err := s.Tx.Query(ctx, `SELECT `+ecCols+` FROM hr.employment_contracts WHERE employee_id=$1 ORDER BY start_date DESC, created_at DESC`, e.ID)
	if err != nil {
		return nil, err
	}
	contracts := []EmpContract{}
	for rows.Next() {
		var c EmpContract
		if err := rows.Scan(&c.ID, &c.Type, &c.Position, &c.StartDate, &c.EndDate, &c.ProbationEnd, &c.NoticeDays, &c.WeeklyHours, &c.Status, &c.SignedOn, &c.DocumentRef, &c.Note); err != nil {
			rows.Close()
			return nil, err
		}
		contracts = append(contracts, c)
	}
	rows.Close()
	body["contracts"] = contracts
	pay := []Pay{}
	rows, err = s.Tx.Query(ctx, `SELECT effective_from, base_salary, allowances FROM payroll.compensation WHERE employee_id=$1 ORDER BY effective_from DESC`, e.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var x Pay
		if err := rows.Scan(&x.EffectiveFrom, &x.BaseSalary, &x.Allowances); err != nil {
			rows.Close()
			return nil, err
		}
		pay = append(pay, x)
	}
	rows.Close()
	body["pay_history"] = pay
	ent, taken, left, err := balance(ctx, s, e.Actor, e.HireDate, time.Now().UTC().Year())
	if err != nil {
		return nil, err
	}
	body["leave"] = map[string]int{"entitlement": ent, "taken": taken, "balance": left}
	return &platform.Result{Status: http.StatusOK, Body: body}, nil
}

func putPrivate(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		DateOfBirth string `json:"date_of_birth"`
		Private
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	var dob any
	if in.DateOfBirth != "" {
		d, err := time.Parse("2006-01-02", in.DateOfBirth)
		if err != nil || d.After(time.Now()) {
			return nil, bad("date_of_birth must be a past date, YYYY-MM-DD")
		}
		dob = d
	}
	ctx := r.Context()
	var id string
	if err := s.Tx.QueryRow(ctx, `SELECT id::text FROM hr.employees WHERE id::text=$1`, r.PathValue("id")).Scan(&id); err != nil {
		if isNF(err) {
			return nil, nf("employee")
		}
		return nil, err
	}
	_, err := s.Tx.Exec(ctx, `INSERT INTO hr.employee_private (employee_id, date_of_birth, national_id, cnps_number, phone, personal_email, address, emergency_name, emergency_phone, bank_name, bank_account, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (employee_id) DO UPDATE SET date_of_birth=$2, national_id=$3, cnps_number=$4, phone=$5, personal_email=$6, address=$7, emergency_name=$8, emergency_phone=$9, bank_name=$10, bank_account=$11, updated_by=$12, updated_at=now()`,
		id, dob, in.NationalID, in.CNPSNumber, in.Phone, in.PersonalEmail, in.Address, in.EmergencyName, in.EmergencyPhone, in.BankName, in.BankAccount, s.Claims.Sub)
	if err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "update_private", "employee", id); err != nil {
		return nil, err
	}
	return employeeFile(r, s)
}

func updateEmployee(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Position   *string `json:"position"`
		Department *string `json:"department"`
		Status     *string `json:"status"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	e, err := scanEmp(s.Tx.QueryRow(r.Context(), `UPDATE hr.employees SET position=COALESCE($2,position), department=COALESCE($3,department), status=COALESCE($4,status) WHERE id::text=$1 RETURNING `+empCols,
		r.PathValue("id"), in.Position, in.Department, in.Status))
	if err != nil {
		if strings.Contains(err.Error(), "status_check") {
			return nil, bad("status must be active or left")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "update", "employee", e.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: e}, nil
}

func optDate(v, field string) (any, error) {
	if v == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, bad(field + " must be YYYY-MM-DD")
	}
	return d, nil
}

func createContract(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var in struct {
		Type         string  `json:"type"`
		Position     string  `json:"position"`
		StartDate    string  `json:"start_date"`
		EndDate      string  `json:"end_date"`
		ProbationEnd string  `json:"probation_end"`
		NoticeDays   *int    `json:"notice_days"`
		WeeklyHours  float64 `json:"weekly_hours"`
		SignedOn     string  `json:"signed_on"`
		Note         string  `json:"note"`
	}
	if err := platform.DecodeBody(r, &in); err != nil {
		return nil, err
	}
	start, err := optDate(in.StartDate, "start_date")
	if err != nil || start == nil {
		return nil, bad("start_date is required (YYYY-MM-DD)")
	}
	end, err := optDate(in.EndDate, "end_date")
	if err != nil {
		return nil, err
	}
	prob, err := optDate(in.ProbationEnd, "probation_end")
	if err != nil {
		return nil, err
	}
	signed, err := optDate(in.SignedOn, "signed_on")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Position) == "" || in.Type == "" {
		return nil, bad("type and position are required")
	}
	notice := 30
	if in.NoticeDays != nil {
		notice = *in.NoticeDays
	}
	if in.WeeklyHours == 0 {
		in.WeeklyHours = 40
	}
	var c EmpContract
	err = s.Tx.QueryRow(r.Context(), `INSERT INTO hr.employment_contracts (employee_id, type, position, start_date, end_date, probation_end, notice_days, weekly_hours, signed_on, note, created_by)
		SELECT id, $2,$3,$4,$5,$6,$7,$8,$9,$10,$11 FROM hr.employees WHERE id::text=$1 RETURNING `+ecCols,
		r.PathValue("id"), in.Type, strings.TrimSpace(in.Position), start, end, prob, notice, in.WeeklyHours, signed, in.Note, s.Claims.Sub).
		Scan(&c.ID, &c.Type, &c.Position, &c.StartDate, &c.EndDate, &c.ProbationEnd, &c.NoticeDays, &c.WeeklyHours, &c.Status, &c.SignedOn, &c.DocumentRef, &c.Note)
	if isNF(err) {
		return nil, nf("employee")
	}
	if err != nil {
		if strings.Contains(err.Error(), "check constraint") {
			return nil, bad("type must be CDI, CDD, internship or consultant; a CDD needs an end date; dates must be in order")
		}
		return nil, err
	}
	if err := s.Audit(r.Context(), "create", "employment_contract", c.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusCreated, Body: c}, nil
}

// activateContract makes a draft the live contract and brings the employee record in line with it.
func activateContract(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	ctx := r.Context()
	var c EmpContract
	var emp string
	err := s.Tx.QueryRow(ctx, `SELECT `+ecCols+`, employee_id::text FROM hr.employment_contracts WHERE id::text=$1 FOR UPDATE`, r.PathValue("id")).
		Scan(&c.ID, &c.Type, &c.Position, &c.StartDate, &c.EndDate, &c.ProbationEnd, &c.NoticeDays, &c.WeeklyHours, &c.Status, &c.SignedOn, &c.DocumentRef, &c.Note, &emp)
	if isNF(err) {
		return nil, nf("contract")
	}
	if err != nil {
		return nil, err
	}
	if c.Status != "draft" {
		return nil, platform.Err(http.StatusConflict, "Not a draft", "only a draft contract can be activated")
	}
	if c.SignedOn == nil {
		return nil, bad("record the date the contract was signed first")
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE hr.employment_contracts SET status='ended' WHERE employee_id=$1 AND status='active'`, emp); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE hr.employment_contracts SET status='active' WHERE id=$1`, c.ID); err != nil {
		return nil, err
	}
	if _, err := s.Tx.Exec(ctx, `UPDATE hr.employees SET position=$2 WHERE id=$1`, emp, c.Position); err != nil {
		return nil, err
	}
	if err := s.Audit(ctx, "activate", "employment_contract", c.ID); err != nil {
		return nil, err
	}
	c.Status = "active"
	return &platform.Result{Status: http.StatusOK, Body: c}, nil
}

func endContract(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	var c EmpContract
	err := s.Tx.QueryRow(r.Context(), `UPDATE hr.employment_contracts SET status='ended', end_date=COALESCE(end_date, current_date) WHERE id::text=$1 AND status='active' RETURNING `+ecCols, r.PathValue("id")).
		Scan(&c.ID, &c.Type, &c.Position, &c.StartDate, &c.EndDate, &c.ProbationEnd, &c.NoticeDays, &c.WeeklyHours, &c.Status, &c.SignedOn, &c.DocumentRef, &c.Note)
	if isNF(err) {
		return nil, platform.Err(http.StatusConflict, "Not active", "only an active contract can be ended")
	}
	if err != nil {
		return nil, err
	}
	if err := s.Audit(r.Context(), "end", "employment_contract", c.ID); err != nil {
		return nil, err
	}
	return &platform.Result{Status: http.StatusOK, Body: c}, nil
}

// exportEmployees is the staff register: work details and the live contract, without personal data.
func exportEmployees(r *http.Request, s *platform.Scope) (*platform.Result, error) {
	rows, err := s.Tx.Query(r.Context(), `SELECT e.full_name, e.actor, e.position, e.department, e.hire_date, e.status, COALESCE(c.type,''), c.start_date, c.end_date
		FROM hr.employees e LEFT JOIN hr.employment_contracts c ON c.employee_id = e.id AND c.status = 'active' ORDER BY e.full_name LIMIT 5000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var name, actor, pos, dep, status, ctype string
		var hire time.Time
		var cs, ce *time.Time
		if err := rows.Scan(&name, &actor, &pos, &dep, &hire, &status, &ctype, &cs, &ce); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"full_name": name, "actor": actor, "position": pos, "department": dep, "hire_date": hire, "status": status, "contract_type": ctype, "contract_start": cs, "contract_end": ce})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reports.CSV("employees", []reports.Col{reports.C("Name", "full_name"), reports.C("Account", "actor"), reports.C("Position", "position"), reports.C("Department", "department"),
		reports.C("Hired", "hire_date"), reports.C("Status", "status"), reports.C("Contract", "contract_type"), reports.C("Contract start", "contract_start"), reports.C("Contract end", "contract_end")}, out)
}
