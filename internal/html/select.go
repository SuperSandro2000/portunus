/*******************************************************************************
*
* Copyright 2019 Stefan Majewsky <majewsky@gmx.net>
*
* This program is free software: you can redistribute it and/or modify it under
* the terms of the GNU General Public License as published by the Free Software
* Foundation, either version 3 of the License, or (at your option) any later
* version.
*
* This program is distributed in the hope that it will be useful, but WITHOUT ANY
* WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
* A PARTICULAR PURPOSE. See the GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License along with
* this program. If not, see <http://www.gnu.org/licenses/>.
*
*******************************************************************************/

package h

import (
	"encoding/hex"
	"fmt"
	"html/template"
	math_rand "math/rand"
	"net/http"
)

//SelectFieldSpec is a FormField where values can be selected from a given set.
//It's rendered as a series of checkboxes.
type SelectFieldSpec struct {
	Name     string
	Label    string
	Options  []SelectOptionSpec
	ReadOnly bool
}

//ReadState implements the FormField interface.
func (f SelectFieldSpec) ReadState(r *http.Request, formState *FormState) {
	if f.ReadOnly {
		return
	}

	isValidValue := make(map[string]bool)
	for _, o := range f.Options {
		isValidValue[o.Value] = true
	}

	s := FieldState{Selected: make(map[string]bool)}
	for _, value := range r.PostForm[f.Name] {
		s.Selected[value] = true
		if !isValidValue[value] {
			s.ErrorMessage = fmt.Sprintf("does not have the option %q", value)
		}
	}
	formState.Fields[f.Name] = &s
}

var selectFieldSnippetReadonly = NewSnippet(`
	<div class="form-row">
		<div class="row-label">{{.Spec.Label}}</div>
		<div class="item-list item-list-readonly">
			{{range .Spec.Options}}
				{{ $unchecked := not (index $.State.Selected .Value) }}
				<span class="item item-{{if $unchecked}}un{{end}}checked">{{.Label}}</span>
			{{end}}
		</div>
	</div>
`)

var selectFieldSnippet = NewSnippet(`
	<div class="form-row">
		<div class="row-label">
			{{.Spec.Label}}
			{{if .State.ErrorMessage}}
				<span class="form-error">{{.State.ErrorMessage}}</span>
			{{end}}
		</div>
		<div class="item-list">
			{{- range $idx, $opt := .Spec.Options -}}
				{{- $id := printf "%s-%d" $.Spec.Name $idx -}}
				<input
					type="checkbox"
					name="{{$.Spec.Name}}"
					id="{{$id}}"
					value="{{$opt.Value}}"
					{{if index $.State.Selected $opt.Value}}checked{{end}}
				/><label for="{{$id}}" class="item">{{$opt.Label}}</label>
			{{- end -}}
		</div>
	</div>
`)

//RenderField implements the FormField interface.
func (f SelectFieldSpec) RenderField(state FormState) template.HTML {
	data := struct {
		Spec  SelectFieldSpec
		State *FieldState
	}{
		Spec:  f,
		State: state.Fields[f.Name],
	}
	if data.State == nil {
		data.State = &FieldState{}
	}

	if f.ReadOnly {
		return selectFieldSnippetReadonly.Render(data)
	}
	return selectFieldSnippet.Render(data)
}

//SelectOptionSpec describes an option that can be selected in a SelectFieldSpec.
type SelectOptionSpec struct {
	Value string
	Label string
}

func getRandomID() string {
	var buf [10]byte
	math_rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
