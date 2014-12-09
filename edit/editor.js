
// this is evaluated at page load time...

var editor = ace.edit("editor");
editor.setTheme("ace/theme/chrome");
editor.setFontSize(12);

var s = editor.getSession();
s.setMode("ace/mode/html");
var rhs = document.getElementById("page")
rhs.srcdoc = s.getValue();

var modified = false;

s.on('change', function (e) {
    if (!modified) {
        document.getElementById("pubbutton").disabled = false;
        document.getElementById("formcomment").disabled = false;
        document.getElementById("pubinfo").innerHTML = "Page is modified";
        modified = true;
    }
    
    rhs.srcdoc = s.getValue();
})

function rename() {
    document.getElementById("formfilename").disabled = false;
    var filename = document.getElementById("formfilename").value;
    document.getElementById("renameinfo").innerHTML = "Old name: " + filename;
}

function publish() {
    console.log("publishing...");
    // see https://developer.mozilla.org/en-US/docs/Web/Guide/HTML/Forms/Sending_forms_through_JavaScript

    var XHR = new XMLHttpRequest();
    var FD = new FormData();
    
    var filename = document.getElementById("formfilename").value;
    var comment = document.getElementById("formcomment").value;
    var body = editor.getSession().getValue();
    console.log("filename = [" + filename + "]");
    console.log("comment = [" + comment + "]");

    FD.append("filename", filename)
    FD.append("comment", comment)
    FD.append("body", body)
    
    XHR.open('POST', '/edit/html')
    XHR.send(FD)
}
