"use client"

import { UserButton } from "@clerk/nextjs"
import { Cloud } from "lucide-react"
import { FileUpload } from "@/components/ui/file-upload"
import { toast } from "sonner"

// Handles a file upload by sending it to a lambda endpoint.
// The endpoint is expected to accept a POST request with a single
// form field named "image" that contains the uploaded file.
const handleFileUpload = async (files: File[]) => {
  const file = files[0];
  if (!file) return;

  const formData = new FormData();
  formData.append("image", file);
  
  try {
    const response = await fetch(process.env.NEXT_PUBLIC_UPLOAD_ENDPOINT!, {
      method: "POST",
      body: formData,
    });

    if (!response.ok) {
      throw new Error("Failed to upload image");
    }

    // Lambda should return the URL of the uploaded image
    const { imageUrl } = await response.json(); 
    console.log("Image URL:", imageUrl);
    toast.success("Image uploaded successfully");
    // ToDo: Add it to recent activity or refresh some UI state
  } catch (error) {
    console.error("Upload error:", error);
    toast.error("Failed to upload image");
  }
};


function DashboardPage() {
  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="border-b bg-white/50 backdrop-blur-sm sticky top-0 z-50">
        <div className="container mx-auto px-4 py-4 flex items-center justify-between">
          {/* Logo and Brand */}
          <div className="flex items-center gap-3">
            <div className="flex items-center justify-center w-10 h-10 bg-blue-100 rounded-lg">
              <Cloud className="w-6 h-6 text-blue-600" />
            </div>
            <div>
              <h1 className="text-xl font-bold text-gray-900">Nimbus</h1>
              {/* <p className="text-sm text-gray-500">Cloud Storage</p> */}
            </div>
          </div>

          {/* User Profile */}
          <div className="flex items-center gap-4">
            <UserButton
              appearance={{
                elements: {
                  avatarBox: "w-10 h-10",
                  userButtonPopoverCard: "shadow-lg",
                },
              }}
              showName={false}
              userProfileMode="modal"
            />
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto px-4 py-8">
        <div className="max-w-4xl mx-auto">
          {/* Welcome Section */}
          <div className="text-center mb-12">
            <h2 className="text-3xl font-bold text-gray-900 mb-4">Welcome to Nimbus</h2>
            <p className="text-lg text-gray-600 max-w-2xl mx-auto">
              Convert your images to public URLs with blazing fast speed!
            </p>
          </div>

          {/* File Upload Section */}
          <div className="bg-white rounded-2xl border border-gray-200 shadow-sm overflow-hidden">
            <div className="p-6 border-b border-gray-100">
              <div className="flex items-center gap-3">
                <div className="flex items-center justify-center w-10 h-10 bg-blue-100 rounded-lg">
                  <Cloud className="w-5 h-5 text-blue-600" />
                </div>
                <div>
                  <h3 className="text-lg font-semibold text-gray-900">Upload Files</h3>
                  <p className="text-sm text-gray-600">Drag and drop your files or click to browse</p>
                </div>
              </div>
            </div>

            <FileUpload
              onChange={handleFileUpload}
            />
          </div>

          {/* Stats Section */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mt-8">
            <div className="bg-white rounded-xl border border-gray-200 p-6 text-center hover:shadow-md transition-shadow">
              <div className="text-2xl font-bold text-blue-600 mb-2">0</div>
              <div className="text-sm text-gray-600">Files Uploaded</div>
            </div>
            <div className="bg-white rounded-xl border border-gray-200 p-6 text-center hover:shadow-md transition-shadow">
              <div className="text-2xl font-bold text-green-600 mb-2">0 MB</div>
              <div className="text-sm text-gray-600">Storage Used</div>
            </div>
            <div className="bg-white rounded-xl border border-gray-200 p-6 text-center hover:shadow-md transition-shadow">
              <div className="text-2xl font-bold text-purple-600 mb-2">0</div>
              <div className="text-sm text-gray-600">Shared Files</div>
            </div>
          </div>

          {/* Recent Activity Section */}
          <div className="mt-8 bg-white rounded-xl border border-gray-200 p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">Recent Activity</h3>
            <div className="text-center py-8 text-gray-500">
              <Cloud className="w-12 h-12 mx-auto mb-3 text-gray-300" />
              <p>No recent activity</p>
              <p className="text-sm">Upload your first file to get started</p>
            </div>
          </div>
        </div>
      </main>
    </div>
  )
};

export default DashboardPage;

